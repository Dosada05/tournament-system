package services

import (
	"github.com/Dosada05/tournament-system/models"
	"github.com/Dosada05/tournament-system/repositories"
)

func CreateMatch(match *models.Match) error {
	return repositories.CreateMatch(match)
}

func GetMatchByID(id int) (*models.Match, error) {
	return repositories.GetMatchByID(id)
}

func UpdateMatch(id int, match *models.Match) error {
	return repositories.UpdateMatch(id, match)
}

func DeleteMatch(id int) error {
	return repositories.DeleteMatch(id)
}
