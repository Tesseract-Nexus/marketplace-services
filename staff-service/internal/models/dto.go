package models

import (
	"time"

	"github.com/google/uuid"
)

// StaffInfoDTO is a safe response DTO for staff information
// This should be used instead of returning the full Staff model in API responses
// to prevent accidental exposure of sensitive fields
type StaffInfoDTO struct {
	ID              uuid.UUID           `json:"id"`
	TenantID        string              `json:"tenantId"`
	Email           string              `json:"email"`
	FirstName       string              `json:"firstName"`
	LastName        string              `json:"lastName"`
	MiddleName      *string             `json:"middleName,omitempty"`
	Role            StaffRole           `json:"role"`
	DepartmentID    *string             `json:"departmentId,omitempty"`
	JobTitle        *string             `json:"jobTitle,omitempty"`
	ProfilePhotoURL *string             `json:"profilePhotoUrl,omitempty"`
	AccountStatus   *StaffAccountStatus `json:"accountStatus,omitempty"`
	EmploymentType  EmploymentType      `json:"employmentType"`
	StartDate       *time.Time          `json:"startDate,omitempty"`
	Timezone        *string             `json:"timezone,omitempty"`
	Locale          *string             `json:"locale,omitempty"`
	IsActive        bool                `json:"isActive"`
	CreatedAt       time.Time           `json:"createdAt"`
	UpdatedAt       time.Time           `json:"updatedAt"`
}

// ToDTO converts a Staff model to a safe StaffInfoDTO
func (s *Staff) ToDTO() *StaffInfoDTO {
	return &StaffInfoDTO{
		ID:              s.ID,
		TenantID:        s.TenantID,
		Email:           s.Email,
		FirstName:       s.FirstName,
		LastName:        s.LastName,
		MiddleName:      s.MiddleName,
		Role:            s.Role,
		DepartmentID:    s.DepartmentID,
		JobTitle:        s.JobTitle,
		ProfilePhotoURL: s.ProfilePhotoURL,
		AccountStatus:   s.AccountStatus,
		EmploymentType:  s.EmploymentType,
		StartDate:       s.StartDate,
		Timezone:        s.Timezone,
		Locale:          s.Locale,
		IsActive:        s.IsActive,
		CreatedAt:       s.CreatedAt,
		UpdatedAt:       s.UpdatedAt,
	}
}

// StaffLoginResponseDTO is a safe response for login
// Replaces returning full Staff model in StaffLoginResponse
type StaffLoginResponseDTO struct {
	AccessToken       string        `json:"accessToken"`
	RefreshToken      string        `json:"refreshToken,omitempty"`
	ExpiresAt         time.Time     `json:"expiresAt"`
	TokenType         string        `json:"tokenType"`
	Staff             *StaffInfoDTO `json:"staff"`
	MustResetPassword bool          `json:"mustResetPassword"`
	SessionID         string        `json:"sessionId"`
}

// StaffListDTO is a safe response for listing staff
type StaffListDTO struct {
	ID              uuid.UUID           `json:"id"`
	TenantID        string              `json:"tenantId"`
	Email           string              `json:"email"`
	FirstName       string              `json:"firstName"`
	LastName        string              `json:"lastName"`
	Role            StaffRole           `json:"role"`
	DepartmentID    *string             `json:"departmentId,omitempty"`
	JobTitle        *string             `json:"jobTitle,omitempty"`
	ProfilePhotoURL *string             `json:"profilePhotoUrl,omitempty"`
	AccountStatus   *StaffAccountStatus `json:"accountStatus,omitempty"`
	IsActive        bool                `json:"isActive"`
}

// ToListDTO converts a Staff model to a list item DTO
func (s *Staff) ToListDTO() *StaffListDTO {
	return &StaffListDTO{
		ID:              s.ID,
		TenantID:        s.TenantID,
		Email:           s.Email,
		FirstName:       s.FirstName,
		LastName:        s.LastName,
		Role:            s.Role,
		DepartmentID:    s.DepartmentID,
		JobTitle:        s.JobTitle,
		ProfilePhotoURL: s.ProfilePhotoURL,
		AccountStatus:   s.AccountStatus,
		IsActive:        s.IsActive,
	}
}

// StaffPublicDTO is a minimal public view of staff (for other services/APIs)
type StaffPublicDTO struct {
	ID              uuid.UUID `json:"id"`
	FirstName       string    `json:"firstName"`
	LastName        string    `json:"lastName"`
	ProfilePhotoURL *string   `json:"profilePhotoUrl,omitempty"`
	DepartmentID    *string   `json:"departmentId,omitempty"`
	JobTitle        *string   `json:"jobTitle,omitempty"`
}

// ToPublicDTO converts a Staff model to a public DTO
func (s *Staff) ToPublicDTO() *StaffPublicDTO {
	return &StaffPublicDTO{
		ID:              s.ID,
		FirstName:       s.FirstName,
		LastName:        s.LastName,
		ProfilePhotoURL: s.ProfilePhotoURL,
		DepartmentID:    s.DepartmentID,
		JobTitle:        s.JobTitle,
	}
}

// StaffSearchResultDTO is used for search results
type StaffSearchResultDTO struct {
	ID            uuid.UUID           `json:"id"`
	Email         string              `json:"email"`
	FirstName     string              `json:"firstName"`
	LastName      string              `json:"lastName"`
	Role          StaffRole           `json:"role"`
	DepartmentID  *string             `json:"departmentId,omitempty"`
	AccountStatus *StaffAccountStatus `json:"accountStatus,omitempty"`
}

// ToSearchResultDTO converts a Staff model to a search result DTO
func (s *Staff) ToSearchResultDTO() *StaffSearchResultDTO {
	return &StaffSearchResultDTO{
		ID:            s.ID,
		Email:         s.Email,
		FirstName:     s.FirstName,
		LastName:      s.LastName,
		Role:          s.Role,
		DepartmentID:  s.DepartmentID,
		AccountStatus: s.AccountStatus,
	}
}
