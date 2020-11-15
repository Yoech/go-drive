package storage

import (
	"fmt"
	"github.com/jinzhu/gorm"
	"go-drive/common"
	"go-drive/common/types"
)

type DriveStorage struct {
	db *DB
}

func NewDriveStorage(db *DB) (*DriveStorage, error) {
	ds := DriveStorage{db: db}
	return &ds, nil
}

func (d *DriveStorage) GetDrives() ([]types.Drive, error) {
	var drivesConfig []types.Drive
	e := d.db.C().Find(&drivesConfig).Error
	return drivesConfig, e
}

func (d *DriveStorage) GetDrive(name string) (types.Drive, error) {
	var config types.Drive
	e := d.db.C().Where("name = ?", name).Find(&config).Error
	if gorm.IsRecordNotFoundError(e) {
		return config, common.NewNotFoundError()
	}
	return config, e
}

func (d *DriveStorage) AddDrive(drive types.Drive) (types.Drive, error) {
	e := d.db.C().Where("name = ?", drive.Name).Find(&types.Drive{}).Error
	if e == nil {
		return types.Drive{},
			common.NewNotAllowedMessageError(fmt.Sprintf("drive '%s' exists", drive.Name))
	}
	if !gorm.IsRecordNotFoundError(e) {
		return types.Drive{}, e
	}
	e = d.db.C().Create(&drive).Error
	return drive, e
}

func (d *DriveStorage) UpdateDrive(name string, drive types.Drive) error {
	drive.Name = name
	return d.db.C().Save(drive).Error
}

func (d *DriveStorage) DeleteDrive(name string) error {
	return d.db.C().Delete(&types.Drive{}, "name = ?", name).Error
}
