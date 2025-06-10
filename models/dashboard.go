package models

type DashboardStats struct {
	UsersTotal        int `json:"users_total"`
	TournamentsTotal  int `json:"tournaments_total"`
	ActiveTournaments int `json:"active_tournaments"`
	SoloMatchesTotal  int `json:"solo_total"`
	TeamMatchesTotal  int `json:"team_total"`
	BannedUsers       int `json:"banned_users"`
}
