package interfaces

type RootConfig interface {
	// GetAuthor returns the author name for copyright attribution.
	GetAuthor() string
	// GetLicense returns the name of the license for the project.
	GetLicense() string
	// GetConfigFile returns the path to the configuration file.
	GetConfigFile() string
}

type Generator[V any] interface {
	Run(config RootConfig, generatorConfig *V) error
}
