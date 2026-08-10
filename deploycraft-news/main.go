
package main

type Feed struct {
    Name string `yaml:"name"`
    URL  string `yaml:"url"`
}

type Config struct {
    Feeds []Feed `yaml:"feeds"`
}
