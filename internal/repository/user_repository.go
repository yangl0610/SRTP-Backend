package repository

import (
	"context"

	"github.com/QSCTech/SRTP-Backend/models"
	"gorm.io/gorm"
)

type UserRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(ctx context.Context, user *models.User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

func (r *UserRepository) GetByID(ctx context.Context, id uint) (*models.User, error) {
	var user models.User
	if err := r.db.WithContext(ctx).First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) GetByPublicID(ctx context.Context, publicID string) (*models.User, error) {
	var user models.User
	if err := r.db.WithContext(ctx).Where("public_id = ?", publicID).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) GetByAuthUID(ctx context.Context, authUID string) (*models.User, error) {
	var user models.User
	if err := r.db.WithContext(ctx).Where("auth_uid = ?", authUID).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) GetFirst(ctx context.Context) (*models.User, error) {
	var user models.User
	if err := r.db.WithContext(ctx).Order("id ASC").First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) Update(ctx context.Context, user *models.User) error {
	return r.db.WithContext(ctx).Save(user).Error
}
