package api

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
)

const (
	azadBaseURL = "https://azadsell.asan-ps.ir"
	azadUser    = "braxton12311"
	azadPass    = "vzha4mYJ2V66"
	testSubUser = "matin_Test"
)

type ConfigDTO struct {
	ID            int    `json:"id" example:"1"`
	Name          string `json:"name" example:"Germany 01"`
	Location      string `json:"location" example:"DE"`
	RouteType     string `json:"route_type" example:"relay"`
	RouteAddress  string `json:"route_address" example:"DE.DOMAIN.IR"`
	BytesTotal    int64  `json:"bytes_total" example:"53687091200"`
	BytesUsed     int64  `json:"bytes_used" example:"1073741824"`
	RemainingDays int    `json:"remaining_days" example:"14"`
	Value         string `json:"value" example:"[Interface]\nPrivateKey = ..."`
}

type azadConfigsResp struct {
	Endpoints []azadEndpoint `json:"endpoints"`
}

type azadEndpoint struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Address   string `json:"address"`
	Port      int    `json:"port"`
	ConfigURL string `json:"config_url"`
}

type azadUserResp struct {
	DataLimit   int64  `json:"data_limit"`
	UsedTraffic int64  `json:"used_traffic"`
	Expire      *int64 `json:"expire"` // Unix timestamp in seconds, or null
}

var (
	cachedToken string
	tokenMu     sync.Mutex
	httpClient  = &http.Client{Timeout: 10 * time.Second}

	countryNames = map[string]string{
		"DE": "Germany",
		"SE": "Sweden",
		"NL": "Netherlands",
		"TR": "Turkey",
		"AM": "Armenia",
		"US": "USA",
	}
)

func RegisterConfigRoutes(app *fiber.App) {
	app.Get("/api/configs", getConfigs())
}

// getConfigs godoc
// @Summary      Get WireGuard Configurations
// @Description  Fetches available server endpoints, user traffic/expiration info, and raw WireGuard configurations
// @Tags         configs
// @Produce      json
// @Success      200  {array}   ConfigDTO
// @Router       /api/configs [get]
// GET /api/configs
func getConfigs() fiber.Handler {
	return func(c fiber.Ctx) error {
		endpoints, err := fetchAzadEndpoints()
		if err != nil {
			return c.Status(502).JSON(fiber.Map{"error": err.Error()})
		}

		userInfo, err := fetchAzadUser()
		if err != nil {
			return c.Status(502).JSON(fiber.Map{"error": err.Error()})
		}

		remDays := calcRemainingDays(userInfo.Expire)

		// 1. Build clean English names, ISO locations & attach quota/time
		resp := make([]ConfigDTO, len(endpoints))
		counts := make(map[string]int)

		for i, ep := range endpoints {
			iso := extractFlagISO(ep.Name)
			counts[iso]++

			country := countryNames[iso]
			if country == "" {
				country = iso
			}

			resp[i] = ConfigDTO{
				ID:            ep.ID,
				Name:          fmt.Sprintf("%s %02d", country, counts[iso]),
				Location:      iso,
				RouteType:     "relay",
				RouteAddress:  ep.Address,
				BytesTotal:    userInfo.DataLimit,
				BytesUsed:     userInfo.UsedTraffic,
				RemainingDays: remDays,
			}
		}

		// 2. Fetch the actual WireGuard .conf files concurrently
		var wg sync.WaitGroup
		for i, ep := range endpoints {
			wg.Add(1)
			go func(idx int, configURL string) {
				defer wg.Done()
				if confText, err := fetchRawText(configURL); err == nil {
					if start := strings.Index(confText, "[Interface]"); start != -1 {
						confText = confText[start:]
					}
					resp[idx].Value = confText
				}
			}(i, ep.ConfigURL)
		}
		wg.Wait()

		return c.JSON(resp)
	}
}

func calcRemainingDays(expireUnix *int64) int {
	if expireUnix == nil || *expireUnix == 0 {
		return 30 // Fallback when user has unlimited/null expire on Azad panel
	}
	diff := time.Until(time.Unix(*expireUnix, 0)).Hours() / 24
	if diff <= 0 {
		return 0
	}
	return int(math.Ceil(diff))
}

func extractFlagISO(rawName string) string {
	runes := []rune(strings.TrimSpace(rawName))
	if len(runes) >= 2 &&
		runes[0] >= 0x1F1E6 && runes[0] <= 0x1F1FF &&
		runes[1] >= 0x1F1E6 && runes[1] <= 0x1F1FF {
		c1 := 'A' + (runes[0] - 0x1F1E6)
		c2 := 'A' + (runes[1] - 0x1F1E6)
		return string([]rune{c1, c2})
	}
	return "UN"
}

func getAzadToken(forceRefresh bool) (string, error) {
	tokenMu.Lock()
	defer tokenMu.Unlock()

	if cachedToken != "" && !forceRefresh {
		return cachedToken, nil
	}

	form := url.Values{}
	form.Set("username", azadUser)
	form.Set("password", azadPass)

	resp, err := httpClient.PostForm(azadBaseURL+"/api/admin/token", form)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var data struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", err
	}

	cachedToken = data.AccessToken
	return cachedToken, nil
}

func fetchAzadEndpoints() ([]azadEndpoint, error) {
	body, err := fetchRawText("/api/wg/user/" + testSubUser + "/configs")
	if err != nil {
		return nil, err
	}

	var parsed azadConfigsResp
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		return nil, err
	}
	return parsed.Endpoints, nil
}

func fetchAzadUser() (*azadUserResp, error) {
	body, err := fetchRawText("/api/user/" + testSubUser)
	if err != nil {
		return nil, err
	}

	var parsed azadUserResp
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		return nil, err
	}
	return &parsed, nil
}

func fetchRawText(path string) (string, error) {
	token, err := getAzadToken(false)
	if err != nil {
		return "", err
	}

	req, _ := http.NewRequest("GET", azadBaseURL+path, nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		token, err = getAzadToken(true)
		if err != nil {
			return "", err
		}
		req2, _ := http.NewRequest("GET", azadBaseURL+path, nil)
		req2.Header.Set("Authorization", "Bearer "+token)
		resp2, err := httpClient.Do(req2)
		if err != nil {
			return "", err
		}
		defer resp2.Body.Close()
		b, _ := io.ReadAll(resp2.Body)
		return string(b), nil
	}

	b, err := io.ReadAll(resp.Body)
	return string(b), err
}
