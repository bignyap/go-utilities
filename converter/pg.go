package converter

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

func ToPgInt4Ptr(v *int) pgtype.Int4 {
	if v == nil {
		return pgtype.Int4{Valid: false}
	}
	return pgtype.Int4{Int32: int32(*v), Valid: true}
}

func ToPgInt4FromTime(t time.Time) pgtype.Int4 {
	return pgtype.Int4{Int32: int32(t.Unix()), Valid: true}
}

func ToPgInt4FromTimePtr(ptr *time.Time) pgtype.Int4 {
	if ptr == nil {
		return pgtype.Int4{Valid: false}
	}
	return ToPgInt4FromTime(*ptr)
}

func ToPgText(ptr *string) pgtype.Text {
	if ptr == nil {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: *ptr, Valid: true}
}

func ToPgBool(ptr *bool) pgtype.Bool {
	if ptr == nil {
		return pgtype.Bool{Valid: false}
	}
	return pgtype.Bool{Bool: *ptr, Valid: true}
}

func FromPgInt4Ptr(v pgtype.Int4) *int {
	if !v.Valid {
		return nil
	}
	val := int(v.Int32)
	return &val
}

func FromPgInt4TimePtr(v pgtype.Int4) *time.Time {
	if !v.Valid {
		return nil
	}
	t := time.Unix(int64(v.Int32), 0)
	return &t
}

func FromPgText(v pgtype.Text) *string {
	if !v.Valid {
		return nil
	}
	return &v.String
}

func FromPgBool(v pgtype.Bool) *bool {
	if !v.Valid {
		return nil
	}
	return &v.Bool
}

func ToPgInt4(ptr *int) pgtype.Int4 {
	if ptr == nil {
		return pgtype.Int4{Valid: false}
	}
	return pgtype.Int4{Int32: int32(*ptr), Valid: true}
}
