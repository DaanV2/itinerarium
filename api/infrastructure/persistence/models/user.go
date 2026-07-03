package models

// Role distinguishes players from GMs. GMs create and manage accounts and
// have unrestricted read access; players are scoped by ownership and group
// membership.
type Role string

const (
	RoleGM     Role = "gm"
	RolePlayer Role = "player"
)

// User is an account holder. GMs create player accounts directly (no
// self-registration) and reset passwords manually.
type User struct {
	Model
	Email        string `gorm:"uniqueIndex;not null" json:"email"`
	PasswordHash string `gorm:"not null" json:"-"`
	Role         Role   `gorm:"not null" json:"role"`
}
