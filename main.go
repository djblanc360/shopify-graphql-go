package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	// "html/template" // for escaping strings
	"log"
	"os"
	"strings" // for sanitizing string properties

	"github.com/joho/godotenv" // https://github.com/joho/godotenv
	"github.com/machinebox/graphql"

	"github.com/gorilla/mux" // for routing

	"context"
)

// "shopify-grahpql/structs" // no longer needed

// remove new lines and escape characters
func sanitizeString(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, `\"`, ``)
	s = strings.TrimSpace(s)
	// s = template.HTMLEscapeString(s)

	return s
}

func products(url string, token string, handle string) (map[string]interface{}, error) {
	// create client
	client := graphql.NewClient(url)

	//make query
	query := `
    query getProductByHandle($handle: String!) {
        productByHandle(handle: $handle) {
            id
            title
            description
            featuredImage {
                url
                altText
            }
            images(first: 10) {
                edges {
                    node {
                        url
                        altText
                    }
                }
            }
            variants(first: 10) {
                edges {
                    node {
                        id
                        title
                        price
                        image {
                            altText
                            url
                        }
                    }
                }
            }
        }
    }`

	// build request
	req := graphql.NewRequest(query)
	req.Var("handle", handle)
	req.Header.Set("X-Shopify-Access-Token", token)

	// just raw JSON
	var respData map[string]interface{}

	// define context for request
	ctx := context.Background()

	if err := client.Run(ctx, req, &respData); err != nil {
		log.Fatalf("failed to fetch product: %v", err)
	}

	// sanitize string properties
	data := respData["productByHandle"].(map[string]interface{})
	if desc, ok := data["description"].(string); ok {
		data["description"] = sanitizeString(desc)
	}

	return respData, nil
}

func collection(url string, token string, handle string) (map[string]interface{}, error) {
	// create client
	client := graphql.NewClient(url)

	// make query
	query := `
    query getCollection($handle: String!) {
        collectionByHandle(handle: $handle) {
            id
            title
            products(first: 5) {
                edges {
                    node {
                        id
                        title
                        handle
                    }
                }
            }
        }
    }`

	// build request
	req := graphql.NewRequest(query)
	req.Var("handle", handle)
	req.Header.Set("X-Shopify-Access-Token", token)

	// define response structure
	var respData map[string]interface{}

	// define context for request
	ctx := context.Background()

	if err := client.Run(ctx, req, &respData); err != nil {
		log.Fatalf("failed to fetch collection: %v", err)
	}

	return respData, nil
}

func collectionProducts(url string, token string, handle string) (string, error) {
	// fetch collection
	collectionResp, err := collection(url, token, handle)
	if err != nil {
		return "", fmt.Errorf("error fetching collection: %v", err)
	}

	// extract collection
	collectionHandle := collectionResp["collectionByHandle"].(map[string]interface{})
	collectionID := collectionHandle["id"].(string)
	collectionTitle := collectionHandle["title"].(string)

	// response structure
	collectionProducts := map[string]interface{}{
		"id":       collectionID,
		"title":    collectionTitle,
		"products": []map[string]interface{}{},
	}

	// iterate over products in collection
	productsData := collectionHandle["products"].(map[string]interface{})["edges"].([]interface{})

	for _, productEdge := range productsData {
		productNode := productEdge.(map[string]interface{})["node"].(map[string]interface{})
		handle := productNode["handle"].(string)

		// fetch product by handle
		productDetails, err := products(url, token, handle)
		if err != nil {
			log.Printf("error fetching product details: %v\n", err)
			continue
		}

		// extract product
		productNodeDetails := productDetails["productByHandle"].(map[string]interface{})
		product := map[string]interface{}{
			"handle":      handle,
			"id":          productNodeDetails["id"].(string),
			"title":       productNodeDetails["title"].(string),
			"description": productNodeDetails["description"].(string),
			"images":      []map[string]interface{}{},
			"variants":    []map[string]interface{}{},
		}

		if featuredImage, ok := productNodeDetails["featuredImage"].(map[string]interface{}); ok {
			product["featuredImage"] = map[string]interface{}{
				"altText": featuredImage["altText"].(string),
				"url":     featuredImage["url"].(string),
			}
		}

		if images, ok := productNodeDetails["images"].(map[string]interface{}); ok {
			imageEdges := images["edges"].([]interface{})
			for _, edge := range imageEdges {
				imageNode := edge.(map[string]interface{})["node"].(map[string]interface{})
				image := map[string]interface{}{
					"altText": imageNode["altText"].(string),
					"url":     imageNode["url"].(string),
				}
				product["images"] = append(product["images"].([]map[string]interface{}), image)
			}
		}

		// extract variants
		variants := productNodeDetails["variants"].(map[string]interface{})["edges"].([]interface{})
		for _, variantEdge := range variants {
			variantNode := variantEdge.(map[string]interface{})["node"].(map[string]interface{})
			variant := map[string]interface{}{
				"handle": handle,
				"id":     variantNode["id"].(string),
				"title":  variantNode["title"].(string),
				"price":  variantNode["price"].(string),
			}

			// if image
			if image, ok := variantNode["image"].(map[string]interface{}); ok {
				variant["image"] = map[string]interface{}{
					"altText": image["altText"].(string),
					"url":     image["url"].(string),
				}
			}

			// append variant to product
			product["variants"] = append(product["variants"].([]map[string]interface{}), variant)
		}

		// append product to collection
		collectionProducts["products"] = append(collectionProducts["products"].([]map[string]interface{}), product)
	}

	// convert collection with products to json
	data, err := json.MarshalIndent(collectionProducts, "", "  ")
	if err != nil {
		return "", fmt.Errorf("error marshalling to JSON: %v", err)
	}

	// json as string
	return string(data), nil
}

func handleCollectionProducts(w http.ResponseWriter, r *http.Request) {
	// get collection handle from request
	params := mux.Vars(r)
	handle := params["handle"]

	// load env variables
	url := os.Getenv("SHOPIFY_URL")
	token := os.Getenv("SHOPIFY_ADMIN_TOKEN")

	// fetch collection products
	json, err := collectionProducts(url, token, handle)
	if err != nil {
		http.Error(w, fmt.Sprintf("error fetching collection products: %v", err), http.StatusInternalServerError)
		return
	}

	// write response
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(json))
}

func main() {
	// load env variables
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatal("error loading .env file")
	}

	r := mux.NewRouter()

	r.HandleFunc("/api/collections/{handle}", handleCollectionProducts).Methods("GET")

	/* // BEFORE ROUTING
	url := os.Getenv("SHOPIFY_URL")
	token := os.Getenv("SHOPIFY_ADMIN_TOKEN")

	// hardcoded collection handle to retrieve products
	handle := "frontpage"
	json, err := collectionProducts(url, token, handle)
	if err != nil {
		log.Fatalf("error fetching collection products: %v", err)
	}

	fmt.Printf("collection with products:\n%s\n", json)
	*/
	// start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Starting server on port %s...", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}
