package main

import (
	"github.com/kmorin72/stock/utils"
	"fmt"
	"encoding/json"
	"time"
	"strconv"
	"net/http"
	"io/ioutil"
	"os"
	"sort"
	"strings"
	"path/filepath"
	"log"
)

var useFilesFirst = true	
var wtdToken = ""

type WorldTradingDataCurrent struct {
	SymbolsRequested int `json:"symbols_requested"`
	SymbolsReturned  int `json:"symbols_returned"`
	Data             []struct {
		Symbol         string `json:"symbol"`
		Name           string `json:"name"`
		Currency       string `json:"currency"`
		Price          string `json:"price"`
		PriceOpen      string `json:"price_open"`
		DayHigh        string `json:"day_high"`
		DayLow         string `json:"day_low"`
		Five2WeekHigh  string `json:"52_week_high"`
		Five2WeekLow   string `json:"52_week_low"`
		DayChange      string `json:"day_change"`
		ChangePct      string `json:"change_pct"`
		CloseYesterday string `json:"close_yesterday"`
		MarketCap      string `json:"market_cap"`
		Volume         string `json:"volume"`
	} `json:"data"`
}

type WorldTradingDataHistory struct {
	Name    string `json:"name"`
	History []struct {
		//Date string `json:"date"`
		Date string `json:"date"`
		Data struct {
			Open   string `json:"open"`
			Close  string `json:"close"`
			High   string `json:"high"`
			Low    string `json:"low"`
			Volume string `json:"volume"`
		} `json:"data"`
	} `json:"history"`
}

type TransactionsData struct {
	Transactions []struct {
		Symbol   string 	`json:"symbol"`
		Type     string 	`json:"type"`
		Date     string 	`json:"date"`
		Quantity int 		`json:"quantity"`
		Price    float64 	`json:"price"`
	} `json:"transactions"`
}

type StockEvent struct {
	Type 		string
	Quantity	int
	Amount		float64
	SplitTo		int
	SplitFrom	int
}

type Stock struct {
	Symbol         		string
	Name           		string
	Currency       		string
	Price          		float64
	FiftyTwoWeekHigh 	float64
	Buys          		map[string]Tx
	Sells          		map[string]Tx
	Dividends      		map[string]Dividend
	Splits         		map[string]Split
	Timeline       		map[string][]StockEvent
	HistoricalData 		WorldTradingDataHistory
	TLR					TimeLineResult
	ROI					ReturnOnInvestment
}

type Split struct {
	Symbol string	`json:"symbol"`
	Date   string 	`json:"date"`
	To     int		`json:"to"`
	From   int		`json:"from"`
}

type SplitData struct {
	Splits []Split
}

type Tx struct {
	Date     string
	Quantity int
	Price    float64
}

type Dividend struct {
	Date   	string 	`json:"date"`
	Amount	float64 `json:"amount"`
}

type DividendData struct {
	Symbol		string		`json:"symbol"`
	Dividends 	[]Dividend	`json:"dividends"`
}

type ReturnOnInvestment struct {
	threeDays	float64
	oneWeek 	float64
	twoWeeks 	float64
	oneMonth 	float64
	twoMonths 	float64
	sixMonth 	float64
	oneyear 	float64
	twoyears 	float64
}

type TimeLineResult struct {
	NumberOfShares 		int
	AveragePrice		float64
	DividendPaid		float64
	DividendPerYear		map[string]float64
	DividendHikes		int
	DividendLastYear	float64
	RealizedGains		float64
}

var Stocks 						= make(map[string]Stock)
var GlobalDividendPerYear_CAD	= make(map[string]float64)
var GlobalDividendPerYear_USD	= make(map[string]float64)
var GlobalDividend1Month_CAD 	float64
var GlobalDividend6Months_CAD 	float64
var GlobalDividend1Year_CAD 	float64
var GlobalDividend1Month_USD 	float64
var GlobalDividend6Months_USD 	float64
var GlobalDividend1Year_USD 	float64


func calculateROISince(price float64, target time.Time, history WorldTradingDataHistory) float64 {

	// iterate the history until you hit the date or something before to get the ROI
	for _, day := range history.History {
		t, _ := time.Parse("2006-01-02", day.Date)

		// if the target is not after, it is the same or before
		if !target.Before(t) {
			oldPrice, _ := strconv.ParseFloat(day.Data.Close, 64)
			return (price/oldPrice - 1) * 100
		}
	}
	return -100
}

// returns -1 as third parameter if unable to get the data
func GetWorldTradingData(symbol string) (WorldTradingDataCurrent, WorldTradingDataHistory) {

	if useFilesFirst {
		println("Getting WDT for " + symbol)
	}

	// handle the current data
	var current WorldTradingDataCurrent
	currentJsonFile := "data/wtd/" + symbol + "-current.json"
	_, err := os.Stat(currentJsonFile)
	currentJsonFileNotExists := os.IsNotExist(err)

	if currentJsonFileNotExists || !useFilesFirst {

		// get it from the site and write the to file
		url := "https://www.worldtradingdata.com/api/v1/stock?symbol=" + symbol + "&api_token=" + wtdToken + "&formatted=false"
		response, err := http.Get(url)
		if err != nil {
			fmt.Println("stock: " + symbol + " - " + err.Error())
			os.Exit(1)
		}
		//if err := json.NewDecoder(response.Body).Decode(&current); err != nil {
		//	fmt.Println("stock: " + symbol + " - " + err.Error())
		//	os.Exit(1)
		//}
		body, err := ioutil.ReadAll(response.Body)
		if err = ioutil.WriteFile(currentJsonFile, body, 0666); err != nil {
			fmt.Println("stock: " + symbol + " - " + err.Error())
			os.Exit(1)
		}
	}

	currentJson, err := ioutil.ReadFile(currentJsonFile)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	err = json.Unmarshal(currentJson, &current)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}


	// handle historical data
	var history WorldTradingDataHistory
	historyJsonFile := "data/wtd/" + symbol + "-history.json"
	_, err = os.Stat(historyJsonFile)
	historyJsonFileNotExists := os.IsNotExist(err)

	if (historyJsonFileNotExists || !useFilesFirst) {

		url := "https://www.worldtradingdata.com/api/v1/history?symbol=" + symbol + "&api_token=" + wtdToken + "&formatted=false"
		response, err := http.Get(url)
		if err != nil {
			fmt.Println("stock: " + symbol + " - " + err.Error())
			os.Exit(1)
		}
		//if err := json.NewDecoder(response.Body).Decode(&history); err != nil {
		//	fmt.Println("stock: " + symbol + " - " + err.Error())
		//	os.Exit(1)
		//}
		body, err := ioutil.ReadAll(response.Body)
		if err = ioutil.WriteFile(historyJsonFile, body, 0666); err != nil {
			fmt.Println("stock: " + symbol + " - " + err.Error())
			os.Exit(1)
		}

	}

	historyJson, err := ioutil.ReadFile(historyJsonFile)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	err = json.Unmarshal(historyJson, &history)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	return current, history
}

func getStock(symbol string) Stock {

	// find the stock if it does not exist, create one.
	if _, isIn := Stocks[symbol]; !isIn {
		var tr TimeLineResult
		var roi ReturnOnInvestment
		var wdh	WorldTradingDataHistory
		Stocks[symbol] = Stock{symbol, symbol, "CAD", 0.0, 0, make(map[string]Tx), make(map[string]Tx), make(map[string]Dividend), make(map[string]Split), make(map[string][]StockEvent), wdh, tr, roi}
	}
	return Stocks[symbol]
}

func addStockEvent(stock Stock, date string, event StockEvent) {
	//t := date.Format("2006-01-02")
	if _, isIn := stock.Timeline[date]; !isIn {
		stock.Timeline[date] = []StockEvent{}
	}
	stock.Timeline[date] = append(stock.Timeline[date], event)
}

func populateStocks() {

	// add the splits
	rawSplits, err := ioutil.ReadFile("./data/splits.json")
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	var splitData SplitData
	json.Unmarshal(rawSplits, &splitData)
	for _, split := range splitData.Splits {
		stock := getStock(split.Symbol)
		stock.Splits[split.Date] = split
		addStockEvent(stock, split.Date, StockEvent{"split", 0, 0, split.To, split.From})
	}

	// add the dividends
	filepath.Walk("./data/dividends", func(path string, info os.FileInfo, err error) error {

		if !info.IsDir() {
			rawDividends, err := ioutil.ReadFile(path)
			if err != nil {
				fmt.Println(err.Error())
				os.Exit(1)
			}
			var dividendData DividendData
			json.Unmarshal(rawDividends, &dividendData)
			for _, dividend := range dividendData.Dividends {

				if strings.Index(dividend.Date, "-") == -1 {
					t, _ := time.Parse("01/02/06", dividend.Date)
					dividend.Date = t.Format("2006-01-02")
				}

				stock := getStock(dividendData.Symbol)
				stock.Dividends[dividend.Date] = dividend
				addStockEvent(stock, dividend.Date, StockEvent{"dividend", 0, dividend.Amount, 0, 0})
			}
		}
		return nil
	})


	// add the transactions
	rawTransactions, err := ioutil.ReadFile("./private/transactions.json")
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	var transactionData TransactionsData
	json.Unmarshal(rawTransactions, &transactionData)
	for _, transaction := range transactionData.Transactions {
		stock := getStock(transaction.Symbol)

		// run the split adjustments for each transaction here ...
		// order the splits from first to last and then apply them on the transactions (thinking about it, the order does not matter, but oh well, code is already there...)
		if len(stock.Splits) > 0 {
			dates := []string{}
			for k, _ := range stock.Splits {
				dates = append(dates, k)
			}
			sort.Strings(dates)
			for _, date := range dates {
				split_t, _ := time.Parse("2006-01-02", date)
				tx_t, _ := time.Parse("2006-01-02", transaction.Date)
				if tx_t.Before(split_t) {
					//println(tx_t.Format("2006-01-02") + " - " + transaction.Date + " is before " + split_t.Format("2006-01-02)"))
					split := stock.Splits[date]
					transaction.Quantity 	= transaction.Quantity * split.To / split.From
					transaction.Price		= transaction.Price * float64(split.From) / float64(split.To)
				}
			}
		}

		if strings.Compare(transaction.Type, "buy") == 0 {
			stock.Buys[transaction.Date] = Tx{transaction.Date, transaction.Quantity, transaction.Price}
			addStockEvent(stock, transaction.Date, StockEvent{"buy", transaction.Quantity, transaction.Price, 0, 0})
		} else {
			stock.Sells[transaction.Date] = Tx{transaction.Date, transaction.Quantity, transaction.Price}
			addStockEvent(stock, transaction.Date, StockEvent{"sell", transaction.Quantity, transaction.Price, 0, 0})
		}
	}

// todo iterate map differently, updates are not making it in here ...
	symbols := []string{}
	for symbol, _ := range Stocks {
		symbols = append(symbols, symbol)
	}

	for _, symbol := range symbols {

		stock, _ := Stocks[symbol]

		// find the first purchase and get the stock with the history since that
		if len((stock.Buys)) == 0 {
			delete(Stocks, stock.Symbol)
			continue
		}

		//firstBuy := "3000-01-01"
		//for k, _ := range stock.Buys {
		//	if strings.Compare(k, firstBuy) == -1 {
		//		firstBuy = k
		//	}
		//}
		current, history := GetWorldTradingData(stock.Symbol)
		stock.Name = current.Data[0].Name
		stock.Currency = current.Data[0].Currency
		stock.Price, _ = strconv.ParseFloat(current.Data[0].Price, 64)
		stock.FiftyTwoWeekHigh, _ = strconv.ParseFloat(current.Data[0].Five2WeekHigh, 64)
		stock.HistoricalData = history

		// get the results based on timeline
		stock = processTimeline(stock)

		// fill in the ROI for
		now := time.Now()

		var roi ReturnOnInvestment
		roi.threeDays 	= calculateROISince(stock.Price, now.AddDate(0, 0, -3), 	stock.HistoricalData)
		roi.oneWeek 	= calculateROISince(stock.Price, now.AddDate(0, 0, -7), 	stock.HistoricalData)
		roi.twoWeeks 	= calculateROISince(stock.Price, now.AddDate(0, 0, -14), 	stock.HistoricalData)
		roi.oneMonth 	= calculateROISince(stock.Price, now.AddDate(0, -1, 0), 	stock.HistoricalData)
		roi.twoMonths 	= calculateROISince(stock.Price, now.AddDate(0, -2, 0), 	stock.HistoricalData)
		roi.sixMonth 	= calculateROISince(stock.Price, now.AddDate(0, -6, 0), 	stock.HistoricalData)
		roi.oneyear 	= calculateROISince(stock.Price, now.AddDate(-1, 0, 0), 	stock.HistoricalData)
		roi.twoyears 	= calculateROISince(stock.Price, now.AddDate(-2, 0, 0), 	stock.HistoricalData)
		stock.ROI = roi

		// todo figure out why this is needed ...
		Stocks[symbol] = stock
	}
}


func processTimeline(stock Stock) Stock {
	firstPurchaseFound	:= false
	AmountInvested		:= 0.0
	LastDividendAmount	:= 0.0
	tr 					:= TimeLineResult{0, 0, 0, make(map[string]float64), 0, 0, 0}


	// create a slice of keys strings
	var keys []string
	for k, _ := range stock.Timeline {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	oneMonthAgo := time.Now().AddDate(0, -1, 0)
	sixMonthAgo := time.Now().AddDate(0, -6, 0)
	oneYearAgo 	:= time.Now().AddDate(-1, 0, 0)
	for _, key := range keys {
		events := stock.Timeline[key]
		for _, event := range events {
			switch event.Type {
			case "buy":
				firstPurchaseFound = true
				tr.AveragePrice = (tr.AveragePrice * float64(tr.NumberOfShares) + event.Amount * float64(event.Quantity)) / (float64(tr.NumberOfShares) + float64(event.Quantity))
				AmountInvested += event.Amount * float64(event.Quantity)
				tr.NumberOfShares += event.Quantity
			case "sell":
				tr.NumberOfShares -= event.Quantity
				tr.RealizedGains += (float64(event.Quantity) * event.Amount) - (float64(event.Quantity) * tr.AveragePrice)
			case "split":
				//tr.NumberOfShares 		= tr.NumberOfShares * event.SplitTo / event.SplitFrom
				//tr.AveragePrice		= tr.AveragePrice * float64(event.SplitFrom) / float64(event.SplitTo)
				//LastDividendAmount	= LastDividendAmount * float64(event.SplitFrom) / float64(event.SplitTo)
			case "dividend":
				if (firstPurchaseFound) {
					date, _ := time.Parse("2006-01-02", key)
					year := (strings.Split(key, "-"))[0]
					payout := float64(tr.NumberOfShares) * event.Amount
					tr.DividendPerYear[year] += payout
					if strings.Compare(stock.Currency, "CAD") == 0 {
						GlobalDividendPerYear_CAD[year] += payout
					} else {
						GlobalDividendPerYear_USD[year] += payout
					}
					if (date.After(oneYearAgo)) {
						tr.DividendLastYear += payout
						if strings.Compare(stock.Currency, "CAD") == 0 {
							GlobalDividend1Year_CAD += payout
						} else {
							GlobalDividend1Year_USD += payout
						}
					}
					if (date.After(sixMonthAgo)) {
						if strings.Compare(stock.Currency, "CAD") == 0 {
							GlobalDividend6Months_CAD += payout
						} else {
							GlobalDividend6Months_USD += payout
						}
					}
					if (date.After(oneMonthAgo)) {
						if strings.Compare(stock.Currency, "CAD") == 0 {
							GlobalDividend1Month_CAD += payout
						} else {
							GlobalDividend1Month_USD += payout
						}
					}
					tr.DividendPaid += payout
					if (event.Amount > (LastDividendAmount + .005)) {
						tr.DividendHikes++
						LastDividendAmount = event.Amount
					}
				}
			default:
				fmt.Println("ERROR - unexpected stock event type.")
			}
		}
	}
	stock.TLR = tr
	return stock
}

func GetStockSummaryHeader() string {
	return "Symbol, Currency, Shares, AvgPrice, BookValue, Price, MarketValue, Divy, 1 year, Hikes, Gain, Gain%, 52WHigh, (% from high), 3d, 7d, 14d, 1m, 2m, 6m, 1y, 2y\n"
}

func GetStockSummaryRow(stock Stock) string {
	tr  := stock.TLR
	roi := stock.ROI

	//return ", , , , , , Divy, DivyHikes, Gain, 3d, 7d, 14d, 1m, 2m, 6m, 1y, 2y"
	bv := float64(tr.NumberOfShares) * tr.AveragePrice
	mv := float64(tr.NumberOfShares) * stock.Price
	gp := (stock.Price/tr.AveragePrice - 1) * 100
	fiftytwop := (stock.Price/stock.FiftyTwoWeekHigh - 1) * 100
	str := fmt.Sprintf(stock.Symbol + ", " + stock.Currency + ", %d, %.2f, %.2f, %.2f, %.2f, %.2f, %.2f, %d, %.2f, %.2f%%, %.2f, %.2f%%, %.2f%%, %.2f%%, %.2f%%, %.2f%%, %.2f%%, %.2f%%, %.2f%%, %.2f%%\n",
		tr.NumberOfShares, tr.AveragePrice, bv, stock.Price, mv, tr.DividendPaid, tr.DividendLastYear, tr.DividendHikes, mv-bv, gp, stock.FiftyTwoWeekHigh, fiftytwop, roi.threeDays, roi.oneWeek, roi.twoWeeks, roi.oneMonth, roi.twoMonths, roi.sixMonth, roi.oneyear, roi.twoyears)
	return str
}

func GetStockDetailsString(stock Stock) string {
	//str := fmt.Sprintf("Symbol          : " + stock.Symbol + "    (" + stock.Currency + ")\n")
	str := fmt.Sprintf("Symbol          : %9s    (" + stock.Currency + ")\n", stock.Symbol)
	if stock.TLR.NumberOfShares > 0 {
		str += fmt.Sprintf("Shares          : %9d\n", stock.TLR.NumberOfShares)
		str += fmt.Sprintf("Average Price   : %9.2f    [%9.2f]\n", stock.TLR.AveragePrice, float64(stock.TLR.NumberOfShares)*stock.TLR.AveragePrice)
		pnl := (stock.Price/stock.TLR.AveragePrice - 1) * 100
		str += fmt.Sprintf("Current Price   : %9.2f    [%8.2f%%]\n", stock.Price, pnl)
		str += fmt.Sprintf("Market Value    : %9.2f    [%9.2f]\n", float64(stock.TLR.NumberOfShares)*stock.Price, float64(stock.TLR.NumberOfShares)*stock.Price - float64(stock.TLR.NumberOfShares)*stock.TLR.AveragePrice)
	} else {
		str += fmt.Sprintf("Current Price   : %9.2f\n", stock.Price)
		// get the average sale price and the last sale price
		lastSale := time.Now().AddDate(-20, 0, 0)
		lastSaleAmount := 0.0
		totalQuantity := 0
		totalSale := 0.0
		for date, tx := range stock.Sells {
			totalSale += float64(tx.Quantity) * tx.Price
			totalQuantity += tx.Quantity
			t, _ := time.Parse("2006-01-02", date)
			if t.After(lastSale) {
				lastSale = t
				lastSaleAmount = tx.Price
			}
		}
		avg := totalSale/float64(totalQuantity)
		avgpnl := (stock.Price/avg - 1) * 100
		str += fmt.Sprintf("Avg Sale Price  : %9.2f    [perf vs sale = %8.2f%% ]\n", avg, avgpnl)
		lspnl := (stock.Price/lastSaleAmount - 1) * 100
		str += fmt.Sprintf("Last Sale Price : %9.2f    [perf vs sale = %8.2f%% ]\n", lastSaleAmount, lspnl)


	}
	if len(stock.Sells) > 0 {
		str += fmt.Sprintf("Realized Gains  : %9.2f\n", stock.TLR.RealizedGains)
	}
	keys := []string{}
	for k, _ := range stock.TLR.DividendPerYear {
		keys = append(keys, k)
	}
	str += fmt.Sprintf("Dividends total : %9.2f\n", stock.TLR.DividendPaid)
	sort.Strings(keys)
	for _, key := range keys {
		value := stock.TLR.DividendPerYear[key]
		str += fmt.Sprintf("Dividends " + key + "  : %9.2f\n", value)
	}
	if stock.TLR.DividendPaid > 0 {
		str += fmt.Sprintf("Dividend Hikes  : %9d\n", stock.TLR.DividendHikes)
	}
	str += "\n=====\n=====\n\n"
	return str
}

func printTimeline(stock Stock) {
	fmt.Println("Stock : " + stock.Symbol)
	// create a slice of keys strings
	var keys []string
	for k, _ := range stock.Timeline {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		events := stock.Timeline[key]
		fmt.Println("    " + key)
		for _, event := range events {
			fmt.Printf("         " + event.Type + " %d @ %.2f split %d:%d\n", event.Quantity, event.Amount, event.SplitTo, event.SplitFrom)
		}
	}
}

func ScreenStocks() {

	stocks := []string{}
	//stocks := []string{"AAPL", "AFN.TO", "AMZN", "ATD.B.TO", "BBD.B.TO", "BCE.TO", "BNS.TO", "CHB.TO", "CNR.TO", "COST", "CSH.UN.TO", "CTC.A.TO", "DIS", "DOL.TO", "ENB.TO", "ENF.TO", "FTS.TO", "GDXJ", "GE", "GOOG", "IPL.TO", "KMI", "MCD", "MRU.TO", "MTN", "NA.TO", "NFLX", "NVDA", "POW.TO", "QSR.TO", "RY.TO", "SHOP.TO", "SBUX", "SJ.TO", "SLF.TO", "TD.TO", "TWTR", "UNH", "V", "WEED.TO", "WSP.TO", "XBB.TO", "XHB.TO"}
	for _, stock := range stocks {

		now := time.Now()
		currentData, historyData := GetWorldTradingData(stock)
		price, _ := strconv.ParseFloat(currentData.Data[0].Price, 64)

		threeDays 	:= calculateROISince(price, now.AddDate(0, 0, -3), historyData)
		oneWeek 	:= calculateROISince(price, now.AddDate(0, 0, -7), historyData)
		twoWeeks 	:= calculateROISince(price, now.AddDate(0, 0, -14), historyData)
		oneMonth 	:= calculateROISince(price, now.AddDate(0, -1, 0), historyData)
		twoMonths 	:= calculateROISince(price, now.AddDate(0, -2, 0), historyData)
		sixMonth 	:= calculateROISince(price, now.AddDate(0, -6, 0), historyData)
		oneyear 	:= calculateROISince(price, now.AddDate(-1, 0, 0), historyData)
		twoyears 	:= calculateROISince(price, now.AddDate(-2, 0, 0), historyData)

		fmt.Printf(stock + "\t\t3 days:  %.2f%%\t\t1 week : %.2f%%\t\t2 weeks : %.2f%%\t\t1 month: %.2f%%\t\t2 months: %.2f%%\t\t6 months: %.2f%%\t\t1 year: %.2f%%\t\t2 years: %.2f%%\n", threeDays, oneWeek, twoWeeks, oneMonth, twoMonths, sixMonth, oneyear, twoyears)
	}
}


func main() {
	
	dir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	var userInputs = utils.LoadConfiguration(dir + "/conf/config.json")
	wtdToken = userInputs.WtdToken
	useFilesFirst = userInputs.UseLocalFiles
	
	populateStocks()

	fmt.Printf("\n\nCAD Dividends Last Month     %.2f\n", 	GlobalDividend1Month_CAD)
	fmt.Printf("CAD Dividends Last 6 Months  %.2f\n", 	GlobalDividend6Months_CAD)
	fmt.Printf("CAD Dividends Last Year      %.2f\n", 	GlobalDividend1Year_CAD)


	fmt.Printf("\nUSD Dividends Last Month     %.2f\n", 	GlobalDividend1Month_USD)
	fmt.Printf("USD Dividends Last 6 Months  %.2f\n", 	GlobalDividend6Months_USD)
	fmt.Printf("USD Dividends Last Year      %.2f\n", 	GlobalDividend1Year_USD)

	fmt.Println("\nCAD Dividend per year")
	keys := []string{}
	for k, _ := range GlobalDividendPerYear_CAD {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		value := GlobalDividendPerYear_CAD[key]
		fmt.Printf("    " + key + "  : %.2f\n", 	value)
	}

	fmt.Println("\nUSD Dividend per year")
	keys = []string{}
	for k, _ := range GlobalDividendPerYear_USD {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		value := GlobalDividendPerYear_USD[key]
		fmt.Printf("    " + key + "  : %.2f\n", 	value)
	}

	stock_summary_str := GetStockSummaryHeader()
	active_stocks_str := ""
	inactive_stocks_str := ""

	keys = []string{}
	for k, _ := range Stocks {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		if Stocks[k].TLR.NumberOfShares > 0 {
			stock_summary_str += GetStockSummaryRow(Stocks[k])
			active_stocks_str += GetStockDetailsString(Stocks[k])
		} else {
			inactive_stocks_str += GetStockDetailsString(Stocks[k])
		}
	}

	ioutil.WriteFile("output/stock_summary.csv", []byte(stock_summary_str), 0644)
	ioutil.WriteFile("output/active_stock_details.txt", []byte(active_stocks_str), 0644)
	ioutil.WriteFile("output/inactive_stock_details.txt", []byte(inactive_stocks_str), 0644)
}







///////////////
//response, err := http.Get("https://www.alphavantage.co/query?function=TIME_SERIES_INTRADAY&symbol=AAPL&interval=1min&apikey=Z0SEJLQK6E3WDNFW")
//if err != nil {
//fmt.Printf("The HTTP request failed with error %s\n", err)
//} else {
//data, _ := ioutil.ReadAll(response.Body)
//fmt.Println(string(data))
//}




//func calculateAveragePrice(stock Stock) float64 {
//
//	var totalCost = 0.0
//	var totalQuantity = 0
//
//	for _, tx := range stock.Transactions {
//		totalQuantity += tx.Quantity
//		totalCost += (float64(tx.Quantity) * tx.Price)
//	}
//	avg := (totalCost/float64(totalQuantity))
//	return avg
//}

//func calculatePNLPerBuy(stock Stock, currentPrice float64) ([]float64, float64) {
//	var quantity int = 0
//	var cost float64 = 0.0
//	var pnls = []float64{}
//	for _, tx := range stock.Transactions {
//		quantity += tx.Quantity
//		cost += tx.Price * float64(tx.Quantity)
//		pnl := ((currentPrice/tx.Price) - 1.0) * 100.0
//		pnls = append(pnls, pnl)
//	}
//	pnl := (currentPrice/(cost/float64(quantity)) - 1) * 100
//	return pnls, pnl
//}

//// get it from the file
//rawHistory, err := ioutil.ReadFile("/Users/kmorin/IdeaProjects/empty/first-go-prog/data/AAPL.json")
//if err != nil {
//	fmt.Println(err.Error())
//	os.Exit(1)
//}
//json.Unmarshal(rawHistory, &historyData)

//fmt.Println(currentData.Data[0].Name + " = " + currentData.Data[0].Price + "(day " + currentData.Data[0].ChangePct + "%%)")

//fmt.Printf("Average price for AAPL = %.2f\n", calculateAveragePrice(allStocks["AAPL"]))
//value, _ := strconv.ParseFloat(currentData.Data[0].Price, 64)
//pnls, pnl := calculatePNLPerBuy(allStocks["AAPL"], value)
//var i int =0
//for _, pnl := range pnls {
//	fmt.Printf("AAPL buy # %d [ %s : %d @ %.2f] => %.2f%%\n", i+1, allStocks["AAPL"].Buys[i].Date.Format("2006-01-02"), allStocks["AAPL"].Buys[i].Quantity, allStocks["AAPL"].Buys[i].Price, pnl)
//	i++
//}
//fmt.Printf("AAPL AVG => %.2f%%\n", pnl)

