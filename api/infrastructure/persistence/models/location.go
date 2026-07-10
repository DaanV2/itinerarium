package models

// Location is a named place a character (or, later, a session) can be
// associated with: a town, building, room, or an entire plane. Plane carries
// multi-plane support — campaigns spanning several planes set it to
// distinguish otherwise identically named places; an empty Plane means the
// campaign's default/material plane.
//
// M1 scopes locations to name + description + plane and a single optional
// association per character. M2 adds location inventories and access control;
// M3 opens editing to anyone who can see the location.
type Location struct {
	Model
	Name        string `gorm:"not null" json:"name"`
	Description string `json:"description"`
	Plane       string `json:"plane"`
}
