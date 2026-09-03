package repositories

import (
	"errors"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lokesh2201013/apperrors"
	"github.com/lokesh2201013/models"
)

type UserRepository struct {
	db *sqlx.DB
}

func NewUserRepository(db *sqlx.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(user *models.User) error {
	if user.UserID == uuid.Nil {
		user.UserID = uuid.New()
	}
	_, err := r.db.NamedExec(`INSERT INTO users (user_id, name, email, password, role, branch, semester) VALUES (:user_id, :name, :email, :password, :role, :branch, :semester)`, map[string]interface{}{"user_id": user.UserID, "name": user.Name, "email": user.Email, "password": user.Password, "role": user.Role, "branch": user.Branch, "semester": user.Semester})
	if err != nil {
		return apperrors.Wrap(apperrors.ErrDatabase, "Could not create user", err)
	}
	return nil
}

func (r *UserRepository) FindByEmail(email string) (models.User, error) {
	var user models.User
	rows, err := r.db.NamedQuery(`SELECT user_id, name, email, password, role, branch, semester FROM users WHERE email = :email`, map[string]interface{}{"email": email})
	if err != nil {
		return models.User{}, apperrors.Wrap(apperrors.ErrDatabase, "Could not fetch user", err)
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return models.User{}, apperrors.Wrap(apperrors.ErrDatabase, "Could not fetch user", err)
		}
		return models.User{}, apperrors.Wrap(apperrors.ErrNotFound, "User not found", errors.New("user not found"))
	}
	if err := rows.Scan(&user.UserID, &user.Name, &user.Email, &user.Password, &user.Role, &user.Branch, &user.Semester); err != nil {
		return models.User{}, apperrors.Wrap(apperrors.ErrDatabase, "Could not fetch user", err)
	}
	return user, nil
}

func (r *UserRepository) FindAdmins() ([]models.User, error) {
	var admins []models.User
	rows, err := r.db.NamedQuery(`SELECT user_id, name, email, password, role, branch, semester FROM users WHERE role = :role`, map[string]interface{}{"role": "admin"})
	if err != nil {
		return nil, apperrors.Wrap(apperrors.ErrDatabase, "Could not retrieve admins", err)
	}
	defer rows.Close()
	for rows.Next() {
		var user models.User
		if err := rows.Scan(&user.UserID, &user.Name, &user.Email, &user.Password, &user.Role, &user.Branch, &user.Semester); err != nil {
			return nil, apperrors.Wrap(apperrors.ErrDatabase, "Could not retrieve admins", err)
		}
		admins = append(admins, user)
	}
	if err := rows.Err(); err != nil {
		return nil, apperrors.Wrap(apperrors.ErrDatabase, "Could not retrieve admins", err)
	}
	return admins, nil
}

func (r *UserRepository) FindStudents(branch string, semester int) ([]models.User, error) {
	var users []models.User
	rows, err := r.db.NamedQuery(`SELECT user_id, name, email, password, role, branch, semester FROM users WHERE role = :role AND branch = :branch AND semester = :semester`, map[string]interface{}{"role": "user", "branch": branch, "semester": semester})
	if err != nil {
		return nil, apperrors.Wrap(apperrors.ErrDatabase, "Could not fetch students", err)
	}
	defer rows.Close()
	for rows.Next() {
		var user models.User
		if err := rows.Scan(&user.UserID, &user.Name, &user.Email, &user.Password, &user.Role, &user.Branch, &user.Semester); err != nil {
			return nil, apperrors.Wrap(apperrors.ErrDatabase, "Could not fetch students", err)
		}
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		return nil, apperrors.Wrap(apperrors.ErrDatabase, "Could not fetch students", err)
	}
	return users, nil
}

func (r *UserRepository) FindByIDs(ids []uuid.UUID) ([]models.User, error) {
	var users []models.User
	if len(ids) == 0 {
		return users, nil
	}
	placeholders := make([]string, len(ids))
	args := make(map[string]interface{}, len(ids))
	for i, id := range ids {
		name := "user_id_" + strconv.Itoa(i)
		placeholders[i] = ":" + name
		args[name] = id
	}
	rows, err := r.db.NamedQuery(`SELECT user_id, name, email, password, role, branch, semester FROM users WHERE user_id IN (`+strings.Join(placeholders, ",")+")", args)
	if err != nil {
		return nil, apperrors.Wrap(apperrors.ErrDatabase, "Could not fetch users", err)
	}
	defer rows.Close()
	for rows.Next() {
		var user models.User
		if err := rows.Scan(&user.UserID, &user.Name, &user.Email, &user.Password, &user.Role, &user.Branch, &user.Semester); err != nil {
			return nil, apperrors.Wrap(apperrors.ErrDatabase, "Could not fetch users", err)
		}
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		return nil, apperrors.Wrap(apperrors.ErrDatabase, "Could not fetch users", err)
	}
	return users, nil
}

func IsUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	value := err.Error()
	for _, needle := range []string{"duplicate key value", "SQLSTATE 23505", "unique constraint"} {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}
