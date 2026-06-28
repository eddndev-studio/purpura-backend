package postgres

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/eddndev-studio/purpura-backend/internal/db"
	"github.com/eddndev-studio/purpura-backend/internal/domain"
	"github.com/eddndev-studio/purpura-backend/internal/ports"
)

// Los enums se persisten como text (05 seccion 2), de modo que sqlc los emite
// como string; el mapeo es un cast simple, sin un segundo tipo enum. uuid y
// timestamptz ya estan override-ados a string y time.Time (05 seccion 6.4).

func toDomainEvent(e db.Event) domain.Event {
	return domain.Event{
		ID:     e.ID,
		UserID: e.UserID,
		Type:   domain.EventType(e.EventType),
		Contact: domain.Contact{
			Name: e.ContactName,
			Ref:  e.ContactRef,
		},
		Location: domain.Location{
			Lat:   e.LocationLat,
			Lng:   e.LocationLng,
			Label: e.LocationLabel,
		},
		Description: e.Description,
		StartsAt:    e.StartsAt,
		Status:      domain.EventStatus(e.EventStatus),
		Reminder:    domain.Reminder(e.ReminderType),
		CreatedAt:   e.CreatedAt,
		UpdatedAt:   e.UpdatedAt,
	}
}

func createEventParams(e *domain.Event) db.CreateEventParams {
	return db.CreateEventParams{
		ID:            e.ID,
		UserID:        e.UserID,
		EventType:     string(e.Type),
		ContactName:   e.Contact.Name,
		ContactRef:    e.Contact.Ref,
		LocationLat:   e.Location.Lat,
		LocationLng:   e.Location.Lng,
		LocationLabel: e.Location.Label,
		Description:   e.Description,
		StartsAt:      e.StartsAt,
		EventStatus:   string(e.Status),
		ReminderType:  string(e.Reminder),
		CreatedAt:     e.CreatedAt,
		UpdatedAt:     e.UpdatedAt,
	}
}

func updateEventParams(e *domain.Event) db.UpdateEventParams {
	return db.UpdateEventParams{
		ID:            e.ID,
		UserID:        e.UserID,
		EventType:     string(e.Type),
		ContactName:   e.Contact.Name,
		ContactRef:    e.Contact.Ref,
		LocationLat:   e.Location.Lat,
		LocationLng:   e.Location.Lng,
		LocationLabel: e.Location.Label,
		Description:   e.Description,
		StartsAt:      e.StartsAt,
		EventStatus:   string(e.Status),
		ReminderType:  string(e.Reminder),
		UpdatedAt:     e.UpdatedAt,
	}
}

func toDomainUser(u db.User) domain.User {
	return domain.User{
		ID:            u.ID,
		Email:         u.Email,
		Nombre:        u.Nombre,
		AuthProvider:  domain.AuthProvider(u.AuthProvider),
		GoogleSub:     u.GoogleSub,
		EmailVerified: u.EmailVerified,
		CreatedAt:     u.CreatedAt,
	}
}

func createUserParams(u *domain.User) db.CreateUserParams {
	return db.CreateUserParams{
		ID:            u.ID,
		Email:         u.Email,
		Nombre:        u.Nombre,
		AuthProvider:  string(u.AuthProvider),
		GoogleSub:     u.GoogleSub,
		EmailVerified: u.EmailVerified,
		CreatedAt:     u.CreatedAt,
	}
}

func toDomainVerificationToken(t db.EmailVerificationToken) ports.VerificationToken {
	return ports.VerificationToken{
		ID:        t.ID,
		UserID:    t.UserID,
		TokenHash: t.TokenHash,
		ExpiresAt: t.ExpiresAt,
		UsedAt:    pgTimePtr(t.UsedAt),
		CreatedAt: t.CreatedAt,
	}
}

func createVerificationTokenParams(t *ports.VerificationToken) db.CreateVerificationTokenParams {
	return db.CreateVerificationTokenParams{
		ID:        t.ID,
		UserID:    t.UserID,
		TokenHash: t.TokenHash,
		ExpiresAt: t.ExpiresAt,
		CreatedAt: t.CreatedAt,
	}
}

// pgTimePtr convierte un timestamptz nullable de pgx a *time.Time (nil si NULL).
func pgTimePtr(ts pgtype.Timestamptz) *time.Time {
	if !ts.Valid {
		return nil
	}
	t := ts.Time
	return &t
}
