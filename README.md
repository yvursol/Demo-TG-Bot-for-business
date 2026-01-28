# Demo-TG-Bot-for-business
A simple Golang bot with limited functionality.
Version GO: go 1.21


neura-market-bot/
     bot.go          ← main bot code
     go.mod          ← dependency description
     run.bat         ← Windows launch script


     ##  How to launch - 3 simple steps

Step 1: Install Go
1. Download from [golang.org](https://golang.org/dl/)
2. Install like a regular program
3. Restart your computer

Step 2: Configure the bot
1. Open `bot.go` in any editor (Notepad, VS Code, Notepad++)
2. **FIND LINE 98** (where `token = "7997269770:AAGttxhAHeyPCnCii3I7izPoxYeT1r_qumY"`)
3. **REPLACE this token with yours** (get from @BotFather)

How to get token:**
- Find @BotFather in Telegram
- Write `/newbot`
- Think up a bot name
- Copy the token (looks like `1234567890:ABCdefGHIjklMnopQRstuVWXYZ`)

4. **FIND LINE 113** (where `adminID = 123456789`)
5. **REPLACE with your Telegram ID** (find out from @userinfobot)

Step 3: Launch
1. Open the folder with the bot
2. Double-click on `run.bat`
3. If everything is OK - you'll see: ` Authorized as @your_bot`

If it doesn't work

### Error: `go: command not found`
- Reinstall Go
- Restart computer
- Check in command line: `go version`

Token error
- Check if you copied the token correctly
- Make sure the bot is created via @BotFather
- Try creating a new bot

Bot doesn't respond
- Write `/start` to the bot
- Check that the bot is running (`run.bat` window is open)
- Restart the bot (close and open `run.bat`)
