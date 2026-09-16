package file

import (
	"context"
	"errors"
	"io"
	"strings"

	"github.com/Radiushina/GophKeeper/gen/filepb"
	"github.com/Radiushina/GophKeeper/internal/domains/user"
	"github.com/Radiushina/GophKeeper/internal/vault"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// GRPC serves streamed file upload/download.
type GRPC struct {
	filepb.UnimplementedFileServiceServer
	svc *Service
	jwt *user.JWT
	log *zap.Logger
}

// NewGRPC builds the file gRPC service.
func NewGRPC(svc *Service, jwt *user.JWT, log *zap.Logger) *GRPC {
	if log == nil {
		log = zap.NewNop()
	}
	return &GRPC{svc: svc, jwt: jwt, log: log}
}

func (g *GRPC) owner(ctx context.Context) (uuid.UUID, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return uuid.Nil, status.Error(codes.Unauthenticated, "user unauthorized")
	}
	raw := ""
	if vals := md.Get("authorization"); len(vals) > 0 {
		raw = vals[0]
	}
	token := strings.TrimSpace(strings.TrimPrefix(raw, "Bearer "))
	if token == "" || g.jwt == nil {
		return uuid.Nil, status.Error(codes.Unauthenticated, "user unauthorized")
	}
	id, err := g.jwt.Parse(token)
	if err != nil {
		return uuid.Nil, status.Error(codes.Unauthenticated, "user unauthorized")
	}
	return id, nil
}

// Upload receives sealed chunks and stores them in S3 one at a time.
func (g *GRPC) Upload(stream grpc.ClientStreamingServer[filepb.UploadChunk, filepb.UploadAck]) error {
	ctx := stream.Context()
	userID, err := g.owner(ctx)
	if err != nil {
		return err
	}
	first, err := stream.Recv()
	if err != nil {
		return status.Error(codes.InvalidArgument, "validate")
	}
	hdr := first.GetHeader()
	if hdr == nil {
		return status.Error(codes.InvalidArgument, "validate")
	}
	if hdr.GetSize() > MaxFileSize {
		return status.Errorf(codes.ResourceExhausted, "%s", SizeError(hdr.GetSize()).Error())
	}
	id, err := uuid.Parse(hdr.GetId())
	if err != nil {
		return status.Error(codes.InvalidArgument, "validate")
	}
	var chunks int32
	for {
		msg, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return status.Error(codes.Canceled, "upload interrupted")
		}
		data := msg.GetData()
		if data == nil {
			return status.Error(codes.InvalidArgument, "validate")
		}
		if int64(data.GetIndex()+1)*int64(ChunkPlaintext) > MaxFileSize+int64(ChunkPlaintext) {
			return status.Errorf(codes.ResourceExhausted, "%s", SizeError(MaxFileSize+1).Error())
		}
		if err := g.svc.PutChunk(ctx, userID, id, data.GetIndex(), data.GetNonce(), data.GetCiphertext()); err != nil {
			g.log.Error("file chunk", zap.Error(err))
			if errors.Is(err, ErrInvalid) {
				return status.Error(codes.InvalidArgument, "validate")
			}
			return status.Error(codes.Internal, "internal server error")
		}
		if data.GetIndex()+1 > chunks {
			chunks = data.GetIndex() + 1
		}
	}
	out, err := g.svc.FinishUpload(ctx, userID, UploadHeader{
		ID:      id,
		Version: hdr.GetVersion(),
		Name:    hdr.GetName(),
		Meta:    hdr.GetMeta(),
		Size:    hdr.GetSize(),
	}, chunks)
	if err != nil {
		g.log.Error("file finish", zap.Error(err))
		switch {
		case errors.Is(err, ErrTooLarge):
			return status.Errorf(codes.ResourceExhausted, "%s", err.Error())
		case errors.Is(err, ErrConflict):
			return status.Error(codes.AlreadyExists, "conflict")
		case errors.Is(err, ErrNotFound):
			return status.Error(codes.NotFound, "not found")
		case errors.Is(err, ErrInvalid):
			return status.Error(codes.InvalidArgument, "validate")
		default:
			return status.Error(codes.Internal, "internal server error")
		}
	}
	return stream.SendAndClose(&filepb.UploadAck{Id: out.ID.String(), Version: out.Version, Chunks: out.ChunkCount})
}

// Download streams sealed chunks. from_index lets a client resume a save.
func (g *GRPC) Download(req *filepb.DownloadReq, stream grpc.ServerStreamingServer[filepb.DownloadChunk]) error {
	ctx := stream.Context()
	userID, err := g.owner(ctx)
	if err != nil {
		return err
	}
	id, err := uuid.Parse(req.GetId())
	if err != nil {
		return status.Error(codes.InvalidArgument, "validate")
	}
	meta, err := g.svc.Get(ctx, userID, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return status.Error(codes.NotFound, "not found")
		}
		return status.Error(codes.Internal, "internal server error")
	}
	if meta.ChunkCount == 0 && len(meta.Ciphertext) > 0 {
		nonce, ct := meta.Nonce, meta.Ciphertext
		if len(nonce) == vault.NonceSize {
			return stream.Send(&filepb.DownloadChunk{
				Index: 0, Nonce: nonce, Ciphertext: ct,
				Name: meta.Name, Meta: meta.Meta, Version: meta.Version, Chunks: 1,
			})
		}
	}
	start := req.GetFromIndex()
	for i := start; i < meta.ChunkCount; i++ {
		sealed, err := g.svc.OpenChunk(ctx, userID, id, i)
		if err != nil {
			return status.Error(codes.Internal, "internal server error")
		}
		if len(sealed) < vault.NonceSize {
			return status.Error(codes.Internal, "internal server error")
		}
		if err := stream.Send(&filepb.DownloadChunk{
			Index: i, Nonce: sealed[:vault.NonceSize], Ciphertext: sealed[vault.NonceSize:],
			Name: meta.Name, Meta: meta.Meta, Version: meta.Version, Chunks: meta.ChunkCount,
		}); err != nil {
			return err
		}
	}
	return nil
}

// UploadStatus returns the next chunk index after a dropped stream.
func (g *GRPC) UploadStatus(ctx context.Context, req *filepb.UploadStatusReq) (*filepb.UploadStatusRes, error) {
	userID, err := g.owner(ctx)
	if err != nil {
		return nil, err
	}
	id, err := uuid.Parse(req.GetId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "validate")
	}
	st, err := g.svc.UploadStatus(ctx, userID, id)
	if err != nil {
		return nil, status.Error(codes.Internal, "internal server error")
	}
	return &filepb.UploadStatusRes{NextIndex: st.NextIndex, Chunks: st.Chunks}, nil
}

// UpdateMeta changes plaintext name/meta only.
func (g *GRPC) UpdateMeta(ctx context.Context, req *filepb.UpdateMetaReq) (*filepb.UploadAck, error) {
	userID, err := g.owner(ctx)
	if err != nil {
		return nil, err
	}
	id, err := uuid.Parse(req.GetId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "validate")
	}
	out, err := g.svc.PatchMeta(ctx, userID, id, req.GetVersion(), req.GetName(), req.GetMeta())
	if err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			return nil, status.Error(codes.NotFound, "not found")
		case errors.Is(err, ErrConflict):
			return nil, status.Error(codes.AlreadyExists, "conflict")
		case errors.Is(err, ErrInvalid):
			return nil, status.Error(codes.InvalidArgument, "validate")
		default:
			return nil, status.Error(codes.Internal, "internal server error")
		}
	}
	return &filepb.UploadAck{Id: out.ID.String(), Version: out.Version, Chunks: out.ChunkCount}, nil
}
