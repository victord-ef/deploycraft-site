package settings

type Feed struct {
	Name string `yaml:"name"`
	URL  string `yaml:"url"`
}

type Configuration struct {
	Feeds []Feed `yaml:"feeds"`
}
