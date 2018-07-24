// Package render implements HTML page creation.
package render

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"

	"appengine"
	"appengine/urlfetch"
)

const (
	yahooApiUrl = `https://query.yahooapis.com/v1/public/yql`
	yqlUrlParam = `select item.condition, wind from weather.forecast where u='c' and woeid = %s`
	owmUrl      = `http://api.openweathermap.org/data/2.5/weather?`
	appID       = `2326504fb9b100bee21400190e4dbe6d`
)

// response structs of Yahoo API start from here
type Wind struct {
	Chill     string `json:"chill"`
	Direction string `json:"direction"`
	Speed     string `json:"speed"`
}

type Condition struct {
	Code string `json:"code"`
	Date string `json:"date"`
	Temp string `json:"temp"`
	Text string `json:"text"`
}

type Item struct {
	Condition Condition `json:"condition"`
}

type Channel struct {
	Wind Wind `json:"wind"`
	Item Item `json:"item"`
}

type ResultsYahoo struct {
	Channel Channel `json:"channel"`
}

type QueryResult struct {
	Count   int          `json:"count"`
	Created string       `json:"created"`
	Lang    string       `json:"lang"`
	Results ResultsYahoo `json:"results"`
}
// response structs of Yahoo API end here
type ResponseYahoo struct {
	Query QueryResult `json:"query"`
}

// response structs of OpenWeatherMap API start from here
type Coord struct {
	Lon float64 `json:"lon"`
	Lat float64 `json:"lat"`
}

type Weather struct {
	ID          int    `json:"id"`
	Main        string `json:"main"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
}

type Main struct {
	Temp     float32 `json:"temp"`
	Pressure float32 `json:"pressure"`
	Humidity float32 `json:"humidity"`
	TempMin  float32 `json:"temp_min"`
	TempMax  float32 `json:"temp_max"`
}

type WindOwm struct {
	Speed float32 `json:"speed"`
	Deg   float32 `json:"deg"`
}

type Clouds struct {
	All float32 `json:"all"`
}

type Sys struct {
	Type    int     `json:"type"`
	ID      int     `json:"id"`
	Message float32 `json:"message"`
	Country string  `json:"country"`
	Sunrise float32 `json:"sunrise"`
	Sunset  float32 `json:"sunset"`
}
// response structs of OpenWeatherMap API end here
type ResponseOwm struct {
	Coord      Coord      `json:"coord"`
	Weather    []*Weather `json:"weather"`
	Base       string     `json:"base"`
	Main       Main       `json:"main"`
	Visibility int        `json:"visibility"`
	Wind       WindOwm    `json:"wind"`
	Clouds     Clouds     `json:"clouds"`
	Dt         int64      `json:"dt"`
	Sys        Sys        `json:"sys"`
	ID         int        `json:"id"`
	Name       string     `json:"name"`
	Cod        int        `json:"cod"`
}

// return result of weather service, wind speed in kph, temperature in Celsius
type WeatherRecord struct {
	WindSpeed   float32 `json:"wind_speed"`
	Temperature float32 `json:"temperature_degrees"`
}

var (
	cityMap    = make(map[string]string)
	owmMap     = make(map[string]string)
	weatherMap = make(map[string]*WeatherRecord)
)

// init reads and compiles the templates in templateDir.
func init() {
	cityMap["sydney"] = "1105779"
	cityMap["melbourne"] = "1103816"
	cityMap["brisbane"] = "1100661"
	owmMap["sydney"] = "sydney,AU"
	owmMap["melbourne"] = "melbourne,AU"
	owmMap["brisbane"] = "brisbane,AU"
	http.HandleFunc("/v1/weather", weatherHandler)
}

// weatherHandler renders the weather page of the site.
func weatherHandler(w http.ResponseWriter, req *http.Request) {
	c := appengine.NewContext(req)

	city := req.URL.Query().Get("city")
	_, ok := cityMap[city]
	if !ok {
		http.Error(w, "Please specify a valid city", 400)
		return
	}

	c.Infof("Query Params are: %v", req.URL.Query())
	var weatherData *WeatherRecord
	var err error
	weatherData, err = GetWeatherFromYahoo(c, city)
	if err != nil {
		c.Infof("Error getting weather data from Yahoo: %v", err)
		weatherData, err = GetWeatherFromOwm(c, city)
		if err != nil {
			c.Infof("Error getting weather data from Openweathermap: %v", err)
			// try to fetch from cached result, if not found, return error
			weatherRec, ok := weatherMap[city]
			if ok {
				weatherData = weatherRec
			} else {
				http.Error(w, "No result found, "+err.Error(), http.StatusInternalServerError)
				return
			}
		}
	}
	// update cached result
	weatherMap[city] = weatherData

	js, err := json.Marshal(weatherData)
	if err != nil {
		c.Infof("Error marshal json response: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
}

// call OpenWeatherMap API
func GetWeatherFromOwm(ctx appengine.Context, city string) (*WeatherRecord, error) {
	params := url.Values{}
	params.Add("q", owmMap[city])
	params.Add("units", "metric")
	params.Add("appid", appID)
	ctx.Infof("Calling Openweathermap API with params: %v", params.Encode())
	// get App Engine http client
	client := urlfetch.Client(ctx)
	resp, err := client.Get(owmUrl + params.Encode())
	if err != nil {
		ctx.Infof("Error calling Openweathermap API: %v", err.Error())
		return nil, err
	}
	ctx.Infof("Openweathermap API response status: %v", resp.Status)
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)

	var r ResponseOwm
	err = json.Unmarshal([]byte(string(body)), &r)
	if err != nil {
		str := fmt.Sprintf("Error parsing json response from Openweathermap: %v", err)
		ctx.Infof(str)
		return nil, err
	}

	wRec := &WeatherRecord{
		WindSpeed:   r.Wind.Speed,
		Temperature: r.Main.Temp,
	}
	return wRec, nil
}

// call Yahoo weather API
func GetWeatherFromYahoo(ctx appengine.Context, city string) (*WeatherRecord, error) {
	cityCode, _ := cityMap[city]
	params := url.Values{}
	params.Add("q", fmt.Sprintf(yqlUrlParam, cityCode))
	params.Add("format", "json")
	params.Add("env", "store://datatables.org/alltableswithkeys")

	req, err := http.NewRequest("POST", yahooApiUrl, bytes.NewBufferString(params.Encode()))
	ctx.Infof("Calling Yahoo API: %v", req.URL.Path)
	// get App Engine http client
	client := urlfetch.Client(ctx)
	resp, err := client.Do(req)
	if err != nil {
		ctx.Infof("Error calling Yahoo API: %v", err.Error())
		return nil, err
	}
	ctx.Infof("Yahoo API response status: %v", resp.Status)
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)

	var r ResponseYahoo
	err = json.Unmarshal([]byte(string(body)), &r)
	if err != nil {
		str := fmt.Sprintf("Error parsing json response from Yahoo: %v", err)
		ctx.Infof(str)
		return nil, err
	}

	windSpeedFloat, err := strconv.ParseFloat(r.Query.Results.Channel.Wind.Speed, 32)
	if err != nil {
		ctx.Infof("Error parsing wind in json: %v", err.Error())
		return nil, err
	}

	tempDegrees, err := strconv.ParseFloat(r.Query.Results.Channel.Item.Condition.Temp, 32)
	if err != nil {
		ctx.Infof("Error parsing temperature in json: %v", err.Error())
		return nil, err
	}
	wRec := &WeatherRecord{
		WindSpeed:   float32(windSpeedFloat),
		Temperature: float32(tempDegrees),
	}
	return wRec, nil
}
