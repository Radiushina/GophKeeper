package file

import (
	"github.com/Radiushina/GophKeeper/gen/oas"
)

func toOAS(n File) oas.File {
	out := oas.File{
		ID:               n.ID,
		Kind:             oas.FileKindFile,
		Version:          n.Version,
		Nonce:            append([]byte(nil), n.Nonce...),
		Name:             oas.NewOptString(n.Name),
		Meta:             oas.NewOptString(n.Meta),
		Ciphertext:       append([]byte(nil), n.Ciphertext...),
		CiphertextSHA256: append([]byte(nil), n.CiphertextSHA256...),
		CreatedAt:        n.CreatedAt.UTC(),
		UpdatedAt:        n.UpdatedAt.UTC(),
	}
	if n.DeletedAt != nil {
		out.DeletedAt = oas.NewOptDateTime(n.DeletedAt.UTC())
	}
	return out
}
