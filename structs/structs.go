package structs

type Variant struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Price string `json:"price"`
	Image struct {
		AltText string `json:"altText"`
		URL     string `json:"url"`
	} `json:"image"`
}

type Product struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Variants    []Variant `json:"variants"`
}

type CollectionProducts struct {
	ID       string    `json:"id"`
	Title    string    `json:"title"`
	Products []Product `json:"products"`
}
