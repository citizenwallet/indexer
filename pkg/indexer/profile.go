package indexer

type Profile struct {
	Address     string `json:"address"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Image       string `json:"image"`
	ImageMedium string `json:"image_medium"`
	ImageSmall  string `json:"image_small"`
}
