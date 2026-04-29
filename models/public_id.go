package models

import "github.com/google/uuid"

func NewPublicID() string {
	return uuid.NewString()
}

func ensurePublicID(publicID *string) {
	if publicID == nil || *publicID != "" {
		return
	}
	*publicID = NewPublicID()
}
