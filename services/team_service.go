package services

import (
	"github.com/Dosada05/tournament-system/models"
	"github.com/Dosada05/tournament-system/repositories"
)

func CreateTeam(team *models.Team) error {
	return repositories.CreateTeam(team)
}

func GetTeamByID(id int) (*models.Team, error) {
	return repositories.GetTeamByID(id)
}

func UpdateTeam(id int, team *models.Team) error {
	return repositories.UpdateTeam(id, team)
}

func DeleteTeam(id int) error {
	return repositories.DeleteTeam(id)
}
