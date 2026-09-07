package note

import (
	"github.com/Radiushina/GophKeeper/gen/oas"
)

func toOAS(n Note) oas.Note {
	out := oas.Note{
		ID:               n.ID,
		Kind:             oas.NoteKindNote,
		Version:          n.Version,
		Nonce:            append([]byte(nil), n.Nonce...),
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
