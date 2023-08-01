package indexer

type Profile struct {
	Account     string `json:"account"`
	Username    string `json:"username"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Image       string `json:"image"`
	ImageMedium string `json:"image_medium"`
	ImageSmall  string `json:"image_small"`
}
