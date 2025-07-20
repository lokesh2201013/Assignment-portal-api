package models


type Error struct{
	ServiceName string `json:"service_name"`
	Message string `json:"message"`
	Description string `json:"description"`
}

