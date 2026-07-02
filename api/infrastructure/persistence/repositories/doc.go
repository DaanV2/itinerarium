// Package repositories holds all database access, one file per entity
// (users.go, characters.go, documents.go, …). Each repository is a struct
// around *persistence.Database with methods returning models — services in
// application/ never build GORM queries themselves.
//
// Shape for a new entity:
//
//	type Users struct{ db *persistence.Database }
//
//	func NewUsers(db *persistence.Database) *Users { return &Users{db: db} }
//
//	func (r *Users) GetByID(ctx context.Context, id string) (*models.User, error) {
//		var user models.User
//		err := r.db.DB().WithContext(ctx).First(&user, "id = ?", id).Error
//		if err != nil {
//			return nil, err
//		}
//		return &user, nil
//	}
package repositories
