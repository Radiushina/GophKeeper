package card

import (
	"github.com/Radiushina/GophKeeper/gen/oas"
)

func toOAS(n Card) oas.Card {
	out := oas.Card{
		ID:               n.ID,
		Kind:             oas.CardKindCard,
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
