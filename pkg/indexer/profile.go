package indexer

type Profile struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Image       string `json:"image"`
	ImageMedium string `json:"image_medium"`
	ImageSmall  string `json:"image_small"`
}
