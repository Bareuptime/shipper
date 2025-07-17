package models

type NomadJobResponse struct {
	EvalID string `json:"EvalID"`
	JobID  string `json:"JobID"`
}

type NomadEvalResponse struct {
	Status string `json:"Status"`
}
