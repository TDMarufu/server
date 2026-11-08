package billing

type Config struct {
	IntegrationKey string
	EncryptionKey  string
	Sandbox        bool

	MonthlyPrice float64
	Currency     string

	ResultURL string
	ReturnURL string
}
