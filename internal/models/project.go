package models

import "time"

type Project struct {
	ID                     int       `json:"id" db:"id"`
	Owner                  Owner     `json:"owner" db:"owner"`
	Languages              []string  `json:"languages" db:"languages"`
	Name                   string    `json:"name" db:"name"` // used as business key
	Company                *string   `json:"company" db:"company"`
	Description            *string   `json:"description" db:"description"`
	MaxDevices             int       `json:"max_devices" db:"max_devices"`
	ProfileImg             string    `json:"profile_img" db:"profile_img"`
	Header                 bool      `json:"header" db:"header"`
	SubType                string    `json:"sub_type" db:"sub_type"`
	Production             bool      `json:"production" db:"production"`
	CityPosterFrequency    int       `json:"city_poster_frequency" db:"city_poster_frequency"`
	AdPosterFrequency      int       `json:"ad_poster_frequency" db:"ad_poster_frequency"`
	CityPosterPlayTime     int       `json:"city_poster_play_time" db:"city_poster_play_time"`
	LoopLength             int       `json:"loop_length" db:"loop_length"`
	SmallbizSupport        bool      `json:"smallbiz_support" db:"smallbiz_support"`
	Proxy                  *string   `json:"proxy" db:"proxy"`
	Address                *string   `json:"address" db:"address"`
	Latitude               string    `json:"latitude" db:"latitude"`
	Longitude              string    `json:"longitude" db:"longitude"`
	IsTransit              bool      `json:"is_transit" db:"is_transit"`
	ScmHealth              bool      `json:"scm_health" db:"scm_health"`
	Priority               int       `json:"priority" db:"priority"`
	Replicas               int       `json:"replicas" db:"replicas"`
	Region                 []int     `json:"region" db:"region"` // array of region IDs
	Status                 string    `json:"status" db:"status"`
	Role                   string    `json:"role" db:"role"`

	// Local fields
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

type Owner struct {
	Username string `json:"username" db:"username"`
}
