package statuswebhooks

import "gorm.io/gorm"

type SelectFilter func(db *gorm.DB) *gorm.DB

func WithUserID(userID string) SelectFilter {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("user_id = ?", userID)
	}
}

func WithExtID(extID string) SelectFilter {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("ext_id = ?", extID)
	}
}

func withActiveForEvent(event string) SelectFilter {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("disabled = ? AND events LIKE ?", false, "%"+event+"%")
	}
}

func newFilter(filters ...SelectFilter) *filter {
	return &filter{filters: filters}
}

type filter struct {
	filters []SelectFilter
}

func (f *filter) apply(db *gorm.DB) *gorm.DB {
	for _, filterFn := range f.filters {
		db = filterFn(db)
	}
	return db
}
