package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	_ "github.com/lib/pq"
)

// Datenstrukturen
type Bestellung struct {
	ID          int64
	UserID      int64
	Username    string
	FirstName   string
	LastName    string
	Telefon     string
	Tarif       string
	Beschreibung string
	Budget      string
	Status      string
	ErstelltAm  time.Time
}

type UserStatus struct {
	Status       string
	TempDaten    map[string]string
	LetzteNachricht int
}

// Konfiguration
type Konfig struct {
	TelegramToken string
	DatenbankURL  string
	AdminID       int64
	Debug         bool
	BotUsername   string
}

// Globale Variablen
var (
	bot    *tgbotapi.BotAPI
	db     *sql.DB
	cfg    *Konfig
	status = make(map[int64]*UserStatus)
	bestellungen []Bestellung
)

func main() {
	// Konfiguration laden
	ladeKonfig()

	// Bot initialisieren
	initBot()

	// Datenbank initialisieren
	initDB()

	// Bot starten
	starteBot()
}

// ========== KONFIGURATION ==========
func ladeKonfig() {
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN ist nicht gesetzt")
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:password@localhost/neura_bot?sslmode=disable"
	}

	adminID, _ := strconv.ParseInt(os.Getenv("ADMIN_ID"), 10, 64)
	if adminID == 0 {
		// GEBEN SIE IHRE TELEGRAM-ID HIER EIN (Zahlen)
		log.Fatal("ADMIN_ID ist nicht gesetzt")
	}

	debug, _ := strconv.ParseBool(os.Getenv("DEBUG"))

	cfg = &Konfig{
		TelegramToken: token,
		DatenbankURL:  dbURL,
		AdminID:       adminID,
		Debug:         debug,
		BotUsername:   "@NeuraMarket_Bot",
	}
}

// ========== BOT INITIALISIERUNG ==========
func initBot() {
	var err error
	bot, err = tgbotapi.NewBotAPI(cfg.TelegramToken)
	if err != nil {
		log.Panic(err)
	}

	bot.Debug = cfg.Debug
	log.Printf("✅ Angemeldet als %s", bot.Self.UserName)
}

// ========== DATENBANK INITIALISIERUNG ==========
func initDB() {
	var err error

	// Datenbankverbindung überspringen wenn nicht benötigt
	if cfg.DatenbankURL == "" || strings.Contains(cfg.DatenbankURL, "localhost") {
		log.Println("⚠️  Bestellungen werden im Speicher gespeichert")
		return
	}

	db, err = sql.Open("postgres", cfg.DatenbankURL)
	if err != nil {
		log.Printf("⚠️  Datenbankverbindung fehlgeschlagen: %v", err)
		log.Println("⚠️  Bestellungen werden im Speicher gespeichert")
		return
	}

	// Verbindung testen
	if err = db.Ping(); err != nil {
		log.Printf("⚠️  Datenbank nicht verfügbar: %v", err)
		db = nil
	} else {
		erstelleTabellen()
		log.Println("✅ Datenbank verbunden")
	}
}

func erstelleTabellen() {
	if db == nil {
		return
	}

	query := `
	CREATE TABLE IF NOT EXISTS bestellungen (
		id SERIAL PRIMARY KEY,
		user_id BIGINT NOT NULL,
		username VARCHAR(255),
		first_name VARCHAR(255),
		last_name VARCHAR(255),
		telefon VARCHAR(50),
		tarif VARCHAR(100) NOT NULL,
		beschreibung TEXT,
		budget VARCHAR(100),
		status VARCHAR(50) DEFAULT 'neu',
		erstellt_am TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	`

	_, err := db.Exec(query)
	if err != nil {
		log.Printf("Fehler beim Erstellen der Tabelle: %v", err)
	}
}

// ========== BOT-HAUPTZYKLUS ==========
func starteBot() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		// Callback-Anfragen verarbeiten (Buttons)
		if update.CallbackQuery != nil {
			handleCallback(update.CallbackQuery)
			continue
		}

		// Nachrichten verarbeiten
		if update.Message == nil {
			continue
		}

		msg := update.Message
		userID := msg.From.ID
		text := strings.TrimSpace(msg.Text)

		// Benutzerstatus initialisieren
		if status[userID] == nil {
			status[userID] = &UserStatus{
				Status:    "bereit",
				TempDaten: make(map[string]string),
			}
		}

		// Befehle verarbeiten
		switch {
		case text == "/start" || text == "/menu" || text == "🏠 Hauptmenü":
			sendeWillkommen(userID)
		case text == "/tarife" || text == "📋 Tarife":
			sendeTarife(userID)
		case text == "/bestellen" || text == "🚀 Bot bestellen":
			starteBestellprozess(userID, "")
		case text == "/hilfe" || text == "❓ Hilfe":
			sendeHilfe(userID)
		case text == "/kontakt" || text == "📞 Kontakt":
			sendeKontakte(userID)
		case text == "/status" || text == "📊 Meine Bestellungen":
			checkBestellstatus(userID)
		default:
			handleBenutzerNachricht(userID, text)
		}
	}
}

// ========== NACHRICHTENVERARBEITUNG ==========
func handleBenutzerNachricht(userID int64, text string) {
	zustand := status[userID]

	switch zustand.Status {
	case "warte_name":
		verarbeiteBenutzername(userID, text)
	case "warte_kontakt":
		verarbeiteBenutzerkontakt(userID, text)
	case "warte_beschreibung":
		verarbeiteBestellbeschreibung(userID, text)
	case "warte_budget":
		verarbeiteBestellbudget(userID, text)
	case "warte_custom_budget":
		verarbeiteCustomBudget(userID, text)
	default:
		handleAllgemeineFrage(userID, text)
	}
}

// ========== CALLBACK-VERARBEITUNG ==========
func handleCallback(callback *tgbotapi.CallbackQuery) {
	userID := callback.From.ID
	data := callback.Data

	// Status initialisieren
	if status[userID] == nil {
		status[userID] = &UserStatus{
			Status:    "bereit",
			TempDaten: make(map[string]string),
		}
	}

	switch {
	// Tarifauswahl
	case data == "tarif_start":
		starteBestellprozess(userID, "🚀 START - 2.900€/Monat")
	case data == "tarif_business":
		starteBestellprozess(userID, "👑 BUSINESS - 5.900€/Monat")
	case data == "tarif_kauf":
		starteBestellprozess(userID, "💎 KAUF - 49.900€")
	case data == "kontakt":
		sendeKontakte(userID)

	// Budgetauswahl
	case data == "budget_2900":
		verarbeiteBestellbudget(userID, "2.900€/Monat")
	case data == "budget_5900":
		verarbeiteBestellbudget(userID, "5.900€/Monat")
	case data == "budget_49900":
		verarbeiteBestellbudget(userID, "49.900€ einmalig")
	case data == "budget_custom":
		frageCustomBudget(userID)

	// Zurück zum Menü
	case data == "hauptmenü":
		sendeWillkommen(userID)
	}

	// Antwort auf Callback
	answer := tgbotapi.NewCallback(callback.ID, "")
	if _, err := bot.Send(answer); err != nil {
		log.Printf("Fehler beim Senden der Callback-Antwort: %v", err)
	}
}

// ========== BESTELLPROZESS ==========
func starteBestellprozess(userID int64, tarif string) {
	zustand := status[userID]
	zustand.Status = "warte_name"

	if tarif != "" {
		zustand.TempDaten["tarif"] = tarif
	}

	msg := tgbotapi.NewMessage(userID, `✨ *Bestellung aufgeben*

*Schritt 1 von 4*
Wie sollen wir Sie ansprechen?
Geben Sie Ihren Namen ein:`)
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = getZurueckZumMenüTastatur()

	if _, err := bot.Send(msg); err != nil {
		log.Printf("Fehler beim Senden der Nachricht: %v", err)
	}
}

func verarbeiteBenutzername(userID int64, name string) {
	zustand := status[userID]
	zustand.Status = "warte_kontakt"
	zustand.TempDaten["name"] = name

	msg := tgbotapi.NewMessage(userID, fmt.Sprintf(`✅ *Perfekt, %s!*

*Schritt 2 von 4*
Wie können wir Sie kontaktieren?
Geben Sie Ihre Telefonnummer oder @username in Telegram ein:`, name))
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = getZurueckZumMenüTastatur()

	if _, err := bot.Send(msg); err != nil {
		log.Printf("Fehler beim Senden der Nachricht: %v", err)
	}
}

func verarbeiteBenutzerkontakt(userID int64, kontakt string) {
	zustand := status[userID]
	zustand.Status = "warte_beschreibung"
	zustand.TempDaten["kontakt"] = kontakt

	msg := tgbotapi.NewMessage(userID, `✅ *Kontakt gespeichert!*

*Schritt 3 von 4*
Beschreiben Sie die Aufgabe für den Bot:
Was soll Ihr zukünftiger Bot tun? (Bestellungen entgegennehmen, Beratung, Zahlung usw.):`)
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = getZurueckZumMenüTastatur()

	if _, err := bot.Send(msg); err != nil {
		log.Printf("Fehler beim Senden der Nachricht: %v", err)
	}
}

func verarbeiteBestellbeschreibung(userID int64, beschreibung string) {
	zustand := status[userID]
	zustand.Status = "warte_budget"
	zustand.TempDaten["beschreibung"] = beschreibung

	msg := tgbotapi.NewMessage(userID, `✅ *Aufgabe verstanden!*

*Schritt 4 von 4*
Wählen Sie ein passendes Budget:`)
	msg.ParseMode = "Markdown"

	// Schöne Tastatur mit Budgetoptionen
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("💰 2.900€/Monat", "budget_2900"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("💎 5.900€/Monat", "budget_5900"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("👑 49.900€ einmalig", "budget_49900"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🎯 Anderes Budget", "budget_custom"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("↩️ Hauptmenü", "hauptmenü"),
		),
	)

	msg.ReplyMarkup = keyboard
	if _, err := bot.Send(msg); err != nil {
		log.Printf("Fehler beim Senden der Nachricht: %v", err)
	}
}

func frageCustomBudget(userID int64) {
	zustand := status[userID]
	zustand.Status = "warte_custom_budget"

	msg := tgbotapi.NewMessage(userID, "💵 *Ihr Budget*\n\nGeben Sie den Betrag in Euro ein:")
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = getZurueckZumMenüTastatur()

	if _, err := bot.Send(msg); err != nil {
		log.Printf("Fehler beim Senden der Nachricht: %v", err)
	}
}

func verarbeiteCustomBudget(userID int64, budget string) {
	verarbeiteBestellbudget(userID, budget+"€")
}

func verarbeiteBestellbudget(userID int64, budget string) {
	zustand := status[userID]

	// Bestellung erstellen
	bestellung := Bestellung{
		UserID:        userID,
		FirstName:     zustand.TempDaten["name"],
		Tarif:         zustand.TempDaten["tarif"],
		Beschreibung:  zustand.TempDaten["beschreibung"],
		Budget:        budget,
		Status:        "neu",
		ErstelltAm:    time.Now(),
	}

	// Bestellung speichern
	speichereBestellung(bestellung)

	// Status zurücksetzen
	zustand.Status = "bereit"
	zustand.TempDaten = make(map[string]string)

	// Bestätigung senden
	sendeBestellbestätigung(userID, bestellung)

	// Administrator benachrichtigen
	benachrichtigeAdmin(bestellung)
}

// ========== BESTELLUNG SPEICHERN ==========
func speichereBestellung(bestellung Bestellung) {
	// Im Speicher speichern
	bestellungen = append(bestellungen, bestellung)

	// In Datenbank speichern, falls vorhanden
	if db != nil {
		query := `INSERT INTO bestellungen 
			(user_id, first_name, tarif, beschreibung, budget, status, erstellt_am) 
			VALUES ($1, $2, $3, $4, $5, $6, $7)`

		_, err := db.Exec(query,
			bestellung.UserID,
			bestellung.FirstName,
			bestellung.Tarif,
			bestellung.Beschreibung,
			bestellung.Budget,
			bestellung.Status,
			bestellung.ErstelltAm)

		if err != nil {
			log.Printf("Fehler beim Speichern in der Datenbank: %v", err)
		}
	}

	log.Printf("🎯 Neue Bestellung: %+v", bestellung)
}

// ========== NACHRICHTEN SENDEN ==========
func sendeWillkommen(userID int64) {
	msg := tgbotapi.NewMessage(userID, `🤖 *NeuraMarket KI-Bots*

*Automatisieren Sie Ihr Geschäft mit KI-Bots für Telegram*

✨ *Unsere Möglichkeiten:*
• 🤖 Intelligente Chat-Bots
• 🛍 Bestellannahme und -bearbeitung
• 💳 Online-Zahlung
• 📊 Analysen und Berichte
• 🚀 Schneller Start (1-3 Tage)

*Wählen Sie eine Aktion:*`)
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = getHauptTastatur()

	if _, err := bot.Send(msg); err != nil {
		log.Printf("Fehler beim Senden der Willkommensnachricht: %v", err)
	}
}

func sendeTarife(userID int64) {
	msg := tgbotapi.NewMessage(userID, `💰 *Unsere Tarife*

Wählen Sie eine passende Option:`)
	msg.ParseMode = "Markdown"

	// Schöne Inline-Tastatur mit Tarifen
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🚀 START - 2.900€/Monat", "tarif_start"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("👑 BUSINESS - 5.900€/Monat", "tarif_business"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("💎 KAUF - 49.900€", "tarif_kauf"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📞 Beratung", "kontakt"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("↩️ Hauptmenü", "hauptmenü"),
		),
	)

	msg.ReplyMarkup = keyboard
	if _, err := bot.Send(msg); err != nil {
		log.Printf("Fehler beim Senden der Tarife: %v", err)
	}
}

func sendeBestellbestätigung(userID int64, bestellung Bestellung) {
	msg := tgbotapi.NewMessage(userID, fmt.Sprintf(`🎉 *Bestellung erfolgreich aufgegeben!*

*Vielen Dank für Ihre Bestellung!*

📋 *Bestelldetails:*
┌─────────────────────────
│ • **Tarif:** %s
│ • **Budget:** %s
│ • **Status:** 🟡 Wird bearbeitet
└─────────────────────────

⏱ *Durchschnittliche Entwicklungszeit:* 1-3 Tage
👨‍💻 *Unser Manager wird sich innerhalb von 15 Minuten bei Ihnen melden*

*Für Kontakt:* %s`, bestellung.Tarif, bestellung.Budget, cfg.BotUsername))
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = getHauptTastatur()

	if _, err := bot.Send(msg); err != nil {
		log.Printf("Fehler beim Senden der Bestätigung: %v", err)
	}
}

func sendeHilfe(userID int64) {
	msg := tgbotapi.NewMessage(userID, `❓ *Hilfe*

*Häufige Fragen:*

🔹 *Wie lange dauert die Entwicklung?*
   └ Vorlagen-Bot: 1-3 Tage
   └ Individueller Bot: 5-10 Tage

🔹 *Wie erfolgt die Zahlung?*
   └ 50%% Vorauszahlung, 50%% nach Fertigstellung
   └ Karte, SEPA, Kryptowährung

🔹 *Gibt es eine Testphase?*
   └ Ja, 3 Tage kostenloses Testen

🔹 *Welche Unterstützung gibt es nach dem Start?*
   └ 1 Monat kostenlose Unterstützung
   └ Danach nach Vereinbarung

📞 *Bei allen Fragen:* `+cfg.BotUsername)
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = getHilfeTastatur()

	if _, err := bot.Send(msg); err != nil {
		log.Printf("Fehler beim Senden der Hilfe: %v", err)
	}
}

func sendeKontakte(userID int64) {
	msg := tgbotapi.NewMessage(userID, `📞 *Kontakte*

*Haupt-Bot:* `+cfg.BotUsername+`
*Neuigkeiten:* @NeuraMarket_news

📧 *E-Mail:* support@neuramarket.de
🌐 *Webseite:* https://neuramarket.de

🕐 *Support-Zeiten:*
┌ Mo-Fr: 9:00-21:00
└ Sa-So: 10:00-18:00

*Wir sind immer für Sie da!* ✨`)
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = getKontaktTastatur()

	if _, err := bot.Send(msg); err != nil {
		log.Printf("Fehler beim Senden der Kontakte: %v", err)
	}
}

func checkBestellstatus(userID int64) {
	// Bestellungen des Benutzers suchen
	var benutzerBestellungen []Bestellung
	for _, bestellung := range bestellungen {
		if bestellung.UserID == userID {
			benutzerBestellungen = append(benutzerBestellungen, bestellung)
		}
	}

	if len(benutzerBestellungen) == 0 {
		msg := tgbotapi.NewMessage(userID, `📭 *Sie haben noch keine Bestellungen*

Beginnen Sie mit der Schaltfläche *"🚀 Bot bestellen"* 👇`)
		msg.ParseMode = "Markdown"
		msg.ReplyMarkup = getHauptTastatur()

		if _, err := bot.Send(msg); err != nil {
			log.Printf("Fehler beim Senden des Status: %v", err)
		}
		return
	}

	text := "📋 *Ihre Bestellungen:*\n\n"
	for i, bestellung := range benutzerBestellungen {
		statusIcon := "🟡"
		if bestellung.Status == "abgeschlossen" {
			statusIcon = "✅"
		} else if bestellung.Status == "in_bearbeitung" {
			statusIcon = "🟠"
		}

		text += fmt.Sprintf(`*Bestellung #%d* %s
┌ Tarif: %s
├ Budget: %s
├ Status: %s %s
└ Datum: %s

`, i+1, statusIcon, bestellung.Tarif, bestellung.Budget, statusIcon, bestellung.Status,
			bestellung.ErstelltAm.Format("02.01.2006 15:04"))
	}

	msg := tgbotapi.NewMessage(userID, text)
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = getHauptTastatur()

	if _, err := bot.Send(msg); err != nil {
		log.Printf("Fehler beim Senden des Status: %v", err)
	}
}

func handleAllgemeineFrage(userID int64, text string) {
	lowerText := strings.ToLower(text)

	switch {
	case strings.Contains(lowerText, "hallo") || strings.Contains(lowerText, "guten tag"):
		sendeWillkommen(userID)
	case strings.Contains(lowerText, "was kostet") || strings.Contains(lowerText, "preis"):
		sendeTarife(userID)
	case strings.Contains(lowerText, "kontakt") || strings.Contains(lowerText, "telefon"):
		sendeKontakte(userID)
	case strings.Contains(lowerText, "hilfe"):
		sendeHilfe(userID)
	case strings.Contains(lowerText, "status") || strings.Contains(lowerText, "bestellung"):
		checkBestellstatus(userID)
	default:
		msg := tgbotapi.NewMessage(userID,
			`🤔 *Ich habe Ihre Frage nicht ganz verstanden*

👉 *Wählen Sie eine der folgenden Aktionen:*`)
		msg.ParseMode = "Markdown"
		msg.ReplyMarkup = getHauptTastatur()
		if _, err := bot.Send(msg); err != nil {
			log.Printf("Fehler beim Senden der Antwort: %v", err)
		}
	}
}

// ========== ADMIN-BENACHRICHTIGUNG ==========
func benachrichtigeAdmin(bestellung Bestellung) {
	msgText := fmt.Sprintf(`🎯 *Neue Bestellung!*

👤 *Kunde:* %s
📞 *Kontakt:* %s
💰 *Tarif:* %s
💎 *Budget:* %s
📝 *Aufgabe:* %s
🕐 *Zeit:* %s
🆔 *ID:* %d

*Status:* 🟡 Neue Anfrage`,
		bestellung.FirstName,
		status[bestellung.UserID].TempDaten["kontakt"],
		bestellung.Tarif,
		bestellung.Budget,
		bestellung.Beschreibung,
		bestellung.ErstelltAm.Format("15:04 02.01.2006"),
		bestellung.UserID)

	msg := tgbotapi.NewMessage(cfg.AdminID, msgText)
	msg.ParseMode = "Markdown"

	if _, err := bot.Send(msg); err != nil {
		log.Printf("Fehler bei der Admin-Benachrichtigung: %v", err)
	}
}

// ========== TASTATUREN ==========
func getHauptTastatur() tgbotapi.ReplyKeyboardMarkup {
	return tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("🚀 Bot bestellen"),
			tgbotapi.NewKeyboardButton("📋 Tarife"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("📞 Kontakt"),
			tgbotapi.NewKeyboardButton("📊 Meine Bestellungen"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("❓ Hilfe"),
		),
	)
}

func getZurueckZumMenüTastatur() tgbotapi.ReplyKeyboardMarkup {
	return tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("🏠 Hauptmenü"),
		),
	)
}

func getHilfeTastatur() tgbotapi.ReplyKeyboardMarkup {
	return tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("📋 Tarife"),
			tgbotapi.NewKeyboardButton("🚀 Bot bestellen"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("📞 Kontakt"),
			tgbotapi.NewKeyboardButton("🏠 Hauptmenü"),
		),
	)
}

func getKontaktTastatur() tgbotapi.ReplyKeyboardMarkup {
	return tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("🚀 Bot bestellen"),
			tgbotapi.NewKeyboardButton("📋 Tarife"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("🏠 Hauptmenü"),
		),
	)
}
