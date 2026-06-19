package config

type Config struct {
	AppEnv                     string
	DatabaseURL                string
	DatabaseName               string
	JWTSecret                  string
	GoogleClientID             string
	CollectionUserName         string
	CollectionExpensesName     string
	CollectionCategoriesName   string
	CollectionTagsName         string
	CollectionAssetsName       string
	CollectionAssetClassesName string
	CollectionPositionLotsName string
	CollectionCashFlowsName    string
	CollectionValuationsName   string
	CollectionPricePointsName  string
	CollectionFxRatesName      string
	DefaultReferenceCurrency   string
	AllowedOrigins             []string
}

func (c *Config) IsDevelopment() bool { return c.AppEnv == "development" }
func (c *Config) IsProduction() bool  { return c.AppEnv == "production" }
func (c *Config) GetDatabaseName() string { return c.DatabaseName }
