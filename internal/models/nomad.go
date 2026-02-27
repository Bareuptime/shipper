package models

type NomadJobResponse struct {
	EvalID string `json:"EvalID"`
	JobID  string `json:"JobID"`
}

type NomadEvalResponse struct {
	ID                string                 `json:"ID"`
	Status            string                 `json:"Status"`
	StatusDescription string                 `json:"StatusDescription"`
	JobID             string                 `json:"JobID"`
	DeploymentID      string                 `json:"DeploymentID"`
	BlockedEval       string                 `json:"BlockedEval"`
	FailedTGAllocs    map[string]interface{} `json:"FailedTGAllocs"`
}

type NomadDeploymentResponse struct {
	ID                string                              `json:"ID"`
	Status            string                              `json:"Status"`
	StatusDescription string                              `json:"StatusDescription"`
	JobID             string                              `json:"JobID"`
	TaskGroups        map[string]NomadTaskGroupDeployment `json:"TaskGroups"`
}

type NomadTaskGroupDeployment struct {
	DesiredCanaries int  `json:"DesiredCanaries"`
	DesiredTotal    int  `json:"DesiredTotal"`
	HealthyAllocs   int  `json:"HealthyAllocs"`
	UnhealthyAllocs int  `json:"UnhealthyAllocs"`
	PlacedAllocs    int  `json:"PlacedAllocs"`
	AutoRevert      bool `json:"AutoRevert"`
}

type NomadJobStopResponse struct {
	EvalID string `json:"EvalID"`
}
