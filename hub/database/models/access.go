package models

import "time"

// AccessGrant deliberately has no wildcard node or action semantics.
type AccessGrant struct {
	UserUUID   string `json:"user_uuid" gorm:"type:varchar(36);primaryKey"`
	ClientUUID string `json:"client_uuid" gorm:"type:varchar(36);primaryKey"`
	Action     string `json:"action" gorm:"type:varchar(40);primaryKey"`
}

type OperationAudit struct {
	ID         uint64    `json:"id" gorm:"primaryKey;autoIncrement"`
	Actor      string    `json:"actor" gorm:"type:varchar(64);index"`
	Action     string    `json:"action" gorm:"type:varchar(80)"`
	ClientUUID string    `json:"client_uuid,omitempty" gorm:"type:varchar(36);index"`
	Outcome    string    `json:"outcome" gorm:"type:varchar(20)"`
	Reason     string    `json:"reason" gorm:"type:varchar(80)"`
	Details    string    `json:"details,omitempty" gorm:"type:text"`
	Time       time.Time `json:"time" gorm:"index"`
}
