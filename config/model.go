package config

type Config struct {
	AppEnv                   string
	DatabaseURL              string
	DatabaseName             string
	JWTSecret                string
	CollectionUserName       string
	CollectionExpensesName   string
	CollectionCategoriesName string
	CollectionTagsName       string
}

// IsDevelopment checks if the current environment is development
func (c *Config) IsDevelopment() bool {
	return c.AppEnv == "development"
}

// IsProduction checks if the current environment is production
func (c *Config) IsProduction() bool {
	return c.AppEnv == "production"
}

// GetDatabaseName returns the appropriate database name based on environment
func (c *Config) GetDatabaseName() string {
	return c.DatabaseName
}
