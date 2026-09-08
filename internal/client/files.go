package client

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/Radiushina/GophKeeper/gen/filepb"
	"github.com/Radiushina/GophKeeper/gen/oas"
	"github.com/Radiushina/GophKeeper/internal/domains/file"
	"github.com/Radiushina/GophKeeper/internal/vault"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// FilePlain is leftover metadata for TUI edit-without-replace.
type FilePlain struct {
	Name string `json:"name"`
	Data []byte `json:"data"`
	Meta string `json:"meta,omitempty"`
}

func (a *App) fileCtx(ctx context.Context) context.Context {
	if a == nil || a.Token() == "" {
		return ctx
	}
	return metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+a.Token())
}

func (a *App) files() (filepb.FileServiceClient, error) {
	if a == nil || a.Files == nil {
		return nil, fmt.Errorf("gRPC files is not configured")
	}
	return a.Files, nil
}

// FileAdd streams a local file in sealed chunks.
func FileAdd(ctx context.Context, app *App, path, meta string) error {
	ack, err := streamUpload(ctx, app, uuid.New(), file.CreateVersion, path, meta)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(os.Stdout, "%s v%d\n%s\n%s\n", ack.GetId(), ack.GetVersion(), filepath.Base(path), meta)
	return err
}

// FileUpdate replaces the blob via stream, or patches meta when path is empty.
func FileUpdate(ctx context.Context, app *App, id uuid.UUID, version int64, path, meta string) error {
	if version == 0 {
		current, err := fetchFile(ctx, app, id)
		if err != nil {
			return err
		}
		version = current.Version
	}
	if path == "" {
		cli, err := app.files()
		if err != nil {
			return err
		}
		ack, err := cli.UpdateMeta(app.fileCtx(ctx), &filepb.UpdateMetaReq{
			Id: id.String(), Version: version, Meta: meta,
		})
		if err != nil {
			return grpcFileErr(err)
		}
		_, err = fmt.Fprintf(os.Stdout, "%s v%d\n%s\n", ack.GetId(), ack.GetVersion(), meta)
		return err
	}
	ack, err := streamUpload(ctx, app, id, version, path, meta)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(os.Stdout, "%s v%d\n%s\n%s\n", ack.GetId(), ack.GetVersion(), filepath.Base(path), meta)
	return err
}

// FileDelete tombstones a file on the server.
func FileDelete(ctx context.Context, app *App, id uuid.UUID) error {
	res, err := app.Client.FileDelete(ctx, oas.FileDeleteParams{ID: id})
	if err != nil {
		return err
	}
	return handleFileRes(app, res, os.Stdout)
}

// FileGet streams chunks to dest (or prints name/meta).
func FileGet(ctx context.Context, app *App, id uuid.UUID, dest string) error {
	name, meta, version, err := downloadFile(ctx, app, id, dest)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(os.Stdout, "%s v%d\n%s\n%s\n", id, version, name, meta)
	return err
}

func downloadFile(ctx context.Context, app *App, id uuid.UUID, dest string) (name, meta string, version int64, err error) {
	cli, err := app.files()
	if err != nil {
		return "", "", 0, err
	}
	stream, err := cli.Download(app.fileCtx(ctx), &filepb.DownloadReq{Id: id.String()})
	if err != nil {
		return "", "", 0, grpcFileErr(err)
	}
	var out *os.File
	if dest != "" {
		out, err = os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
		if err != nil {
			return "", "", 0, err
		}
		defer out.Close()
	}
	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", "", 0, grpcFileErr(err)
		}
		name, meta, version = chunk.GetName(), chunk.GetMeta(), chunk.GetVersion()
		if out == nil {
			continue
		}
		plain, err := openChunk(app, chunk.GetNonce(), chunk.GetCiphertext())
		if err != nil {
			return "", "", 0, err
		}
		if _, err := out.Write(plain); err != nil {
			return "", "", 0, err
		}
	}
	return name, meta, version, nil
}

// FileList prints file metadata without downloading bodies.
func FileList(ctx context.Context, app *App) error {
	res, err := app.Client.ListFiles(ctx, oas.ListFilesParams{})
	if err != nil {
		return err
	}
	list, ok := res.(*oas.FileListRes)
	if !ok {
		return fileMsg(res)
	}
	for i := range list.Items {
		if err := printFile(app, os.Stdout, list.Items[i]); err != nil {
			return err
		}
	}
	return nil
}

func writeFileAdd(ctx context.Context, app *App, path, meta string) error {
	_, err := streamUpload(ctx, app, uuid.New(), file.CreateVersion, path, meta)
	return err
}

func writeFileUpdate(ctx context.Context, app *App, id uuid.UUID, path, meta string, keep FilePlain) error {
	current, err := fetchFile(ctx, app, id)
	if err != nil {
		return err
	}
	if path == "" {
		cli, err := app.files()
		if err != nil {
			return err
		}
		_, err = cli.UpdateMeta(app.fileCtx(ctx), &filepb.UpdateMetaReq{
			Id: id.String(), Version: current.Version, Name: keep.Name, Meta: meta,
		})
		return grpcFileErr(err)
	}
	_, err = streamUpload(ctx, app, id, current.Version, path, meta)
	return err
}

func writeFileDelete(ctx context.Context, app *App, id uuid.UUID) error {
	res, err := app.Client.FileDelete(ctx, oas.FileDeleteParams{ID: id})
	if err != nil {
		return err
	}
	return handleFileRes(app, res, io.Discard)
}

func checkFileSize(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	if info.Size() > file.MaxFileSize {
		return info.Size(), file.SizeError(info.Size())
	}
	return info.Size(), nil
}

func streamUpload(ctx context.Context, app *App, id uuid.UUID, version int64, path, meta string) (*filepb.UploadAck, error) {
	size, err := checkFileSize(path)
	if err != nil {
		return nil, err
	}
	cli, err := app.files()
	if err != nil {
		return nil, err
	}
	ctx = app.fileCtx(ctx)
	st, err := cli.UploadStatus(ctx, &filepb.UploadStatusReq{Id: id.String()})
	if err != nil {
		st = &filepb.UploadStatusRes{}
	}
	ack, err := sendFileStream(ctx, cli, app, id, version, path, meta, size, st.GetNextIndex())
	if err == nil {
		return ack, nil
	}
	// one resume after a dropped stream
	st, stErr := cli.UploadStatus(ctx, &filepb.UploadStatusReq{Id: id.String()})
	if stErr != nil || st.GetNextIndex() == 0 {
		return nil, err
	}
	return sendFileStream(ctx, cli, app, id, version, path, meta, size, st.GetNextIndex())
}

func sendFileStream(ctx context.Context, cli filepb.FileServiceClient, app *App, id uuid.UUID, version int64, path, meta string, size int64, from int32) (*filepb.UploadAck, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if from > 0 {
		if _, err := f.Seek(int64(from)*int64(file.ChunkPlaintext), io.SeekStart); err != nil {
			return nil, err
		}
	}
	stream, err := cli.Upload(ctx)
	if err != nil {
		return nil, grpcFileErr(err)
	}
	if err := stream.Send(&filepb.UploadChunk{Payload: &filepb.UploadChunk_Header{Header: &filepb.UploadHeader{
		Id: id.String(), Version: version, Name: filepath.Base(path), Meta: meta, Size: size, ResumeFrom: from,
	}}}); err != nil {
		return nil, grpcFileErr(err)
	}
	buf := make([]byte, file.ChunkPlaintext)
	idx := from
	for {
		n, readErr := io.ReadFull(f, buf)
		if n > 0 {
			nonce, ct, err := sealBytes(app, buf[:n])
			if err != nil {
				return nil, err
			}
			if err := stream.Send(&filepb.UploadChunk{Payload: &filepb.UploadChunk_Data{Data: &filepb.UploadData{
				Index: idx, Nonce: nonce, Ciphertext: ct,
			}}}); err != nil {
				return nil, grpcFileErr(err)
			}
			idx++
		}
		if readErr == io.EOF || readErr == io.ErrUnexpectedEOF {
			break
		}
		if readErr != nil {
			return nil, readErr
		}
	}
	ack, err := stream.CloseAndRecv()
	if err != nil {
		return nil, grpcFileErr(err)
	}
	return ack, nil
}

func sealBytes(app *App, plain []byte) (nonce, ciphertext []byte, err error) {
	key := app.VaultKey()
	if len(key) != vault.VKSize {
		return nil, nil, fmt.Errorf("login first")
	}
	return vault.Seal(key, plain)
}

func openChunk(app *App, nonce, ciphertext []byte) ([]byte, error) {
	key := app.VaultKey()
	if len(key) != vault.VKSize {
		return nil, fmt.Errorf("login first")
	}
	return vault.Open(key, append(nonce, ciphertext...))
}

func fetchFile(ctx context.Context, app *App, id uuid.UUID) (oas.File, error) {
	res, err := app.Client.FileGet(ctx, oas.FileGetParams{ID: id})
	if err != nil {
		return oas.File{}, err
	}
	n, ok := res.(*oas.File)
	if !ok {
		return oas.File{}, fileMsg(res)
	}
	return *n, nil
}

func handleFileRes(app *App, res any, w io.Writer) error {
	if n, ok := res.(*oas.File); ok {
		return printFile(app, w, *n)
	}
	return fileMsg(res)
}

func printFile(_ *App, w io.Writer, n oas.File) error {
	if w == io.Discard {
		return nil
	}
	deleted := ""
	if n.DeletedAt.IsSet() {
		deleted = " deleted"
	}
	name := n.Name.Value
	meta := n.Meta.Value
	_, err := fmt.Fprintf(w, "%s v%d%s\n%s\n%s\n", n.ID, n.Version, deleted, name, meta)
	return err
}

func grpcFileErr(err error) error {
	if err == nil {
		return nil
	}
	st, ok := status.FromError(err)
	if !ok {
		return err
	}
	if st.Code() == codes.ResourceExhausted || st.Code() == codes.InvalidArgument ||
		st.Code() == codes.AlreadyExists || st.Code() == codes.NotFound ||
		st.Code() == codes.Unauthenticated {
		return fmt.Errorf("%s", st.Message())
	}
	return fmt.Errorf("%s", st.Message())
}

func fileMsg(res any) error {
	switch v := res.(type) {
	case *oas.FileCreateBadRequest:
		return fmt.Errorf("%s", v.Msg)
	case *oas.FileCreateConflict:
		return fmt.Errorf("%s", v.Msg)
	case *oas.FileCreateUnauthorized:
		return fmt.Errorf("%s", v.Msg)
	case *oas.FileCreateInternalServerError:
		return fmt.Errorf("%s", v.Msg)
	case *oas.FileUpdateBadRequest:
		return fmt.Errorf("%s", v.Msg)
	case *oas.FileUpdateConflict:
		return fmt.Errorf("%s", v.Msg)
	case *oas.FileUpdateUnauthorized:
		return fmt.Errorf("%s", v.Msg)
	case *oas.FileUpdateNotFound:
		return fmt.Errorf("%s", v.Msg)
	case *oas.FileUpdateInternalServerError:
		return fmt.Errorf("%s", v.Msg)
	case *oas.FileDeleteUnauthorized:
		return fmt.Errorf("%s", v.Msg)
	case *oas.FileDeleteNotFound:
		return fmt.Errorf("%s", v.Msg)
	case *oas.FileDeleteInternalServerError:
		return fmt.Errorf("%s", v.Msg)
	case *oas.FileGetUnauthorized:
		return fmt.Errorf("%s", v.Msg)
	case *oas.FileGetNotFound:
		return fmt.Errorf("%s", v.Msg)
	case *oas.FileGetInternalServerError:
		return fmt.Errorf("%s", v.Msg)
	case *oas.ListFilesBadRequest:
		return fmt.Errorf("%s", v.Msg)
	case *oas.ListFilesUnauthorized:
		return fmt.Errorf("%s", v.Msg)
	case *oas.ListFilesInternalServerError:
		return fmt.Errorf("%s", v.Msg)
	default:
		return fmt.Errorf("unexpected response %T", res)
	}
}
