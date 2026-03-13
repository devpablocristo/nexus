package domain

type ToolSecret struct {
	ToolID         string
	SecretType     string
	KeyName        string
	PlaintextValue string
	Enabled        bool
}
