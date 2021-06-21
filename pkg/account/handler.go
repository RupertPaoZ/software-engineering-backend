package account

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"net/http"
	"net/smtp"
	"os"
	"regexp"
	"strconv"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/AsterNighT/software-engineering-backend/api"
	"github.com/AsterNighT/software-engineering-backend/pkg/utils"
	jwt "github.com/dgrijalva/jwt-go"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

type AccountHandler struct{}

// @Summary create and account based on email(as id), type, name and password
// @Description will check primarykey other, then add to accountList if possible
// @Tags Account
// @Produce json
// @Param Email path string true "user e-mail"
// @Param Type path string true "user type"
// @Param Name path string true "user name"
// @Param Passwd path string true "user password"
// @Success 200 {string} api.ReturnedData{data=nil}
// @Failure 400 {string} api.ReturnedData{data=nil}
// @Router /account/create [POST]
func (h *AccountHandler) CreateAccount(c echo.Context) error {
	type RequestBody struct {
		Email string `json:"email" validate:"required"`

		Type      AcountType `json:"type" validate:"required"`
		FirstName string     `json:"firstname" validate:"required"`
		LastName  string     `json:"lastname" validate:"required"`
		Passwd    string     `json:"passwd" validate:"required"`
	}

	var body RequestBody
	if err := utils.ExtractDataWithValidating(c, &body); err != nil {
		return c.JSON(http.StatusBadRequest, api.Return("error", err))
	}
	if ok, _ := regexp.MatchString(`^\w+@\w+[.\w+]+$`, body.Email); !ok {
		return c.JSON(http.StatusBadRequest, api.Return("Invalid E-mail Address", nil))
	}
	if body.Type != PatientType && body.Type != DoctorType && body.Type != AdminType {
		return c.JSON(http.StatusBadRequest, api.Return("Invalid Account Type", nil))
	}
	if len(body.Passwd) < accountPasswdLen {
		return c.JSON(http.StatusBadRequest, api.Return("Invalid Password Length", nil))
	}

	db, _ := c.Get("db").(*gorm.DB)
	if err := db.Where("email = ?", body.Email).First(&Account{}).Error; err == nil {
		return c.JSON(http.StatusBadRequest, api.Return("E-Mail or AccountID occupied", nil))
	}

	account := Account{
		Email: body.Email,

		Type:      body.Type,
		FirstName: body.FirstName,
		LastName:  body.LastName,
		Passwd:    body.Passwd,
	}
	account.Token, _ = account.GenerateToken()
	account.HashPassword()

	// if account.Type == "doctor" {
	// 	doctor := Doctor{AccountID: account.ID}
	// 	if result := db.Create(&doctor); result.Error != nil {
	// 		return c.JSON(http.StatusBadRequest, api.Return("DB error", result.Error))
	// 	}
	// } else if account.Type == "patient" {
	// 	patient := Patient{AccountID: account.ID}
	// 	if result := db.Create(&patient); result.Error != nil {
	// 		return c.JSON(http.StatusBadRequest, api.Return("DB error", result.Error))
	// 	}
	// }

	if result := db.Create(&account); result.Error != nil {
		return c.JSON(http.StatusBadRequest, api.Return("DB error", result.Error))
	}

	cookie := http.Cookie{
		Name:     "token",
		Value:    account.Token,
		Expires:  time.Now().Add(7 * 24 * time.Hour),
		Path:     "/api",
		SameSite: http.SameSiteNoneMode,
		Secure:   true,
	}
	c.SetCookie(&cookie)

	return c.JSON(http.StatusOK, api.Return("Created", echo.Map{
		"account":      account,
		"cookie_token": account.Token,
	}))
}

// @Summary check email's existense
// @Description
// @Tags Account
// @Produce json
// @Param Email path string true "user e-mail"
// @Param Passwd path string true "user password"
// @Success 200 {string} api.ReturnedData{data=nil}
// @Failure 400 {string} api.ReturnedData{data=nil}
// @Router /account/checkemail [POST]
func (h *AccountHandler) CheckEmail(c echo.Context) error {
	type RequestBody struct {
		Email string `json:"email" validate:"required"`
	}
	var body RequestBody

	if err := utils.ExtractDataWithValidating(c, &body); err != nil {
		return c.JSON(http.StatusBadRequest, api.Return("error", err))
	}

	if ok, _ := regexp.MatchString(`^\w+@\w+[.\w+]+$`, body.Email); !ok {
		return c.JSON(http.StatusBadRequest, api.Return("Invalid E-mail Address", nil))
	}

	db, _ := c.Get("db").(*gorm.DB)
	var account Account
	if err := db.Where("email = ?", body.Email).First(&account).Error; err != nil { // not found
		return c.JSON(http.StatusBadRequest, api.Return("E-Mail", echo.Map{"emailok": false}))
	}
	return c.JSON(http.StatusOK, api.Return("E-Mail", echo.Map{"emailok": true}))

}

// @Summary login using email and passwd
// @Description
// @Tags Account
// @Produce json
// @Param Email path string true "user e-mail"
// @Param Passwd path string true "user password"
// @Success 200 {string} api.ReturnedData{data=nil}
// @Failure 400 {string} api.ReturnedData{data=nil}
// @Router /account/login [POST]
func (h *AccountHandler) LoginAccount(c echo.Context) error {
	type RequestBody struct {
		Email  string `json:"email" validate:"required"`
		Passwd string `json:"passwd" validate:"required"`
	}
	var body RequestBody

	if err := utils.ExtractDataWithValidating(c, &body); err != nil {
		return c.JSON(http.StatusBadRequest, api.Return("error", err))
	}

	if ok, _ := regexp.MatchString(`^\w+@\w+[.\w+]+$`, body.Email); !ok {
		return c.JSON(http.StatusBadRequest, api.Return("Invalid E-mail Address", nil))
	}
	if len(body.Passwd) < accountPasswdLen {
		return c.JSON(http.StatusBadRequest, api.Return("Invalid Password Length", nil))
	}

	db, _ := c.Get("db").(*gorm.DB)
	var account Account
	if err := db.Where("email = ?", body.Email).First(&account).Error; err != nil { // not found
		return c.JSON(http.StatusBadRequest, api.Return("E-Mail", echo.Map{"emailok": false}))
	}
	if bcrypt.CompareHashAndPassword([]byte(account.Passwd), []byte(body.Passwd)) != nil {
		return c.JSON(http.StatusBadRequest, api.Return("Wrong Password", nil))
	}

	account.Token, _ = account.GenerateToken()
	cookie := http.Cookie{
		Name:     "token",
		Value:    account.Token,
		Expires:  time.Now().Add(7 * 24 * time.Hour),
		Path:     "/api",
		SameSite: http.SameSiteNoneMode,
		Secure:   true,
	}
	c.SetCookie(&cookie)

	if result := db.Model(&Account{}).Where("id = ?", account.ID).Update("token", account.Token); result.Error != nil {
		return c.JSON(http.StatusBadRequest, api.Return("DB error", result.Error))
	}

	return c.JSON(http.StatusOK, api.Return("Logged in", echo.Map{
		"account":      account,
		"cookie_token": account.Token,
	}))
}

// @Summary logout using cookie
// @Description
// @Tags Account
// @Produce json
// @Success 200 {string} api.ReturnedData{data=nil}
// @Failure 400 {string} api.ReturnedData{data=nil}
// @Router /account/logout [POST]
func (h *AccountHandler) LogoutAccount(c echo.Context) error {
	cookie, err := c.Cookie("token")
	if err != nil || cookie.Value == "" {
		return c.JSON(http.StatusBadRequest, api.Return("Not Logged in", nil))
	}
	cookie.Value = ""
	cookie.Expires = time.Unix(0, 0)
	// cookie.Expires = time.Now().Add(7 * 24 * time.Hour)
	cookie.Path = "/api"
	c.SetCookie(cookie)

	return c.JSON(http.StatusOK, api.Return("Account logged out", nil))
}

// @Summary the interface of modifying password
// @Description
// @Tags Account
// @Produce json
// @Param Email path string true "user e-mail"
// @Param Passwd path string true "user password (the new one)"
// @Success 200 {string} api.ReturnedData{data=nil}
// @Failure 400 {string} api.ReturnedData{data=nil}
// @Router /account/modifypasswd [POST]
func (h *AccountHandler) ModifyPasswd(c echo.Context) error {
	type RequestBody struct {
		Email     string `json:"email" validate:"required"`
		Passwd    string `json:"passwd" validate:"required"`
		NewPasswd string `json:"newpasswd" validate:"required"`
	}
	var body RequestBody

	if err := utils.ExtractDataWithValidating(c, &body); err != nil {
		return c.JSON(http.StatusBadRequest, api.Return("error", err))
	}

	if ok, _ := regexp.MatchString(`^\w+@\w+[.\w+]+$`, body.Email); !ok {
		return c.JSON(http.StatusBadRequest, api.Return("Invalid E-mail Address", nil))
	}

	// Check old passwd
	db, _ := c.Get("db").(*gorm.DB)
	var account Account
	if err := db.Where("email = ?", body.Email).First(&account).Error; err != nil { // not found
		return c.JSON(http.StatusBadRequest, api.Return("E-Mail", echo.Map{"emailok": false}))
	}
	if bcrypt.CompareHashAndPassword([]byte(account.Passwd), []byte(body.Passwd)) != nil {
		return c.JSON(http.StatusBadRequest, api.Return("Wrong Password", nil))
	}

	if len(body.NewPasswd) < accountPasswdLen {
		return c.JSON(http.StatusBadRequest, api.Return("Invalid Password Length", nil))
	}

	account.Passwd = body.NewPasswd
	account.HashPassword()

	if result := db.Model(&Account{}).Where("id = ?", account.ID).Update("passwd", account.Passwd); result.Error != nil {
		return c.JSON(http.StatusBadRequest, api.Return("DB error", result.Error))
	}

	return c.JSON(http.StatusOK, api.Return("Successfully modified", nil))
}

// @Summary the interface of sending email to reset password
// @Description can only be called during logged-in status since there is no password check
// @Tags Account
// @Produce json
// @Param Email path string true "user e-mail"
// @Success 200 {string} api.ReturnedData{data=nil}
// @Failure 400 {string} api.ReturnedData{data=echo.Map{"authCode": authCode}}
// @Router /account/sendemail [POST]
func (h *AccountHandler) SendEmail(c echo.Context) error {
	type RequestBody struct {
		Email string `json:"email" validate:"required"`
	}
	var body RequestBody

	if err := utils.ExtractDataWithValidating(c, &body); err != nil {
		return c.JSON(http.StatusBadRequest, api.Return("error", err))
	}

	if ok, _ := regexp.MatchString(`^\w+@\w+[.\w+]+$`, body.Email); !ok {
		return c.JSON(http.StatusBadRequest, api.Return("Invalid E-mail Address", nil))
	}

	// Check old passwd
	db, _ := c.Get("db").(*gorm.DB)
	var account Account
	if err := db.Where("email = ?", body.Email).First(&account).Error; err != nil { // not found
		return c.JSON(http.StatusBadRequest, api.Return("E-Mail", echo.Map{"emailok": false}))
	}

	if result := db.Model(&Account{}).Where("id = ?", account.ID).Update("passwd", account.Passwd); result.Error != nil {
		return c.JSON(http.StatusBadRequest, api.Return("DB error", result.Error))
	}

	authCode := ""
	for i := 0; i < 6; i++ {
		nBig, _ := rand.Int(rand.Reader, big.NewInt(10))
		authCode += string("0123456789"[nBig.Int64()])
	}
	c.Logger().Debug(authCode)

	if tmp := db.Model(&Account{}).Where("id = ?", account.ID).Update("auth_code", authCode); tmp.Error != nil {
		return c.JSON(http.StatusBadRequest, api.Return("DB error", tmp.Error))
	}

	emailServerHost := os.Getenv("EMAIL_SERVER_HOST")
	emailServerPort := os.Getenv("EMAIL_SERVER_PORT")
	emailUser := os.Getenv("EMAIL_USER")
	emailPasswd := os.Getenv("EMAIL_PASSWD")
	expireMin, _ := strconv.Atoi(os.Getenv("EMAIL_VALID_MIN"))

	if tmp := db.Model(&Account{}).Where("id = ?", account.ID).Update("auth_code_expires", time.Now().Add(time.Duration(expireMin)*time.Minute)); tmp.Error != nil {
		return c.JSON(http.StatusBadRequest, api.Return("DB error", tmp.Error))
	}

	auth := smtp.PlainAuth("", emailUser, emailPasswd, emailServerHost)
	to := body.Email
	msg := []byte("From: \"MediConnect\" <noreply@mediconnect.com>\n" +
		"To: " + to + "\n" +
		"Subject: MediConnect Account Reset\n" +
		"Content-Type: text/plain; charset=\"UTF-8\"\n" +
		"\n" +
		"Your verification code is " + authCode + " (Only valid in " + strconv.Itoa(expireMin) + " minutes)\n")
	if err := smtp.SendMail(emailServerHost+":"+emailServerPort, auth, emailUser, []string{to}, msg); err != nil {
		return c.JSON(http.StatusOK, api.Return("Email server error", echo.Map{"err": err, "msg": msg}))
	}

	return c.JSON(http.StatusOK, api.Return("Successfully send reset email", nil))
}

// @Summary check email's existense
// @Description
// @Tags Account
// @Produce json
// @Param Email path string true "user e-mail"
// @Param AuthCode path string true "given auth code"
// @Success 200 {string} api.ReturnedData{data=echo.Map{"authcodeok": false}}
// @Failure 400 {string} api.ReturnedData{data=echo.Map{"authcodeok": true}}
// @Router /account/checkauthcode [POST]
func (h *AccountHandler) CheckAuthCode(c echo.Context) error {
	type RequestBody struct {
		Email    string `json:"email" validate:"required"`
		AuthCode string `json:"authcode" validate:"required"`
	}
	var body RequestBody

	if err := utils.ExtractDataWithValidating(c, &body); err != nil {
		return c.JSON(http.StatusBadRequest, api.Return("error", err))
	}

	if ok, _ := regexp.MatchString(`^\w+@\w+[.\w+]+$`, body.Email); !ok {
		return c.JSON(http.StatusBadRequest, api.Return("Invalid E-mail Address", nil))
	}

	// Check authcode
	db, _ := c.Get("db").(*gorm.DB)
	var account Account
	if err := db.Where("email = ?", body.Email).First(&account).Error; err != nil { // not found
		return c.JSON(http.StatusBadRequest, api.Return("E-Mail", echo.Map{"emailok": false}))
	}
	if account.AuthCode == body.AuthCode && time.Now().Before(account.AuthCodeExpires) {
		return c.JSON(http.StatusOK, api.Return("AuthCode", echo.Map{"authcodeok": true}))
	}
	return c.JSON(http.StatusBadRequest, api.Return("AuthCode", echo.Map{"authcodeok": false}))

}

// @Summary the interface of reset password
// @Description
// @Tags Account
// @Produce json
// @Param Email path string true "user e-mail"
// @Param AuthCode path string true "given auth code"
// @Param Passwd path string true "user password (the new one)"
// @Success 200 {string} api.ReturnedData{data=nil}
// @Failure 400 {string} api.ReturnedData{data=nil}
// @Router /account/resetpasswd [POST]
func (h *AccountHandler) ResetPasswd(c echo.Context) error {
	type RequestBody struct {
		Email     string `json:"email" validate:"required"`
		AuthCode  string `json:"authcode" validate:"required"`
		NewPasswd string `json:"newpasswd" validate:"required"`
	}
	var body RequestBody

	if err := utils.ExtractDataWithValidating(c, &body); err != nil {
		return c.JSON(http.StatusBadRequest, api.Return("error", err))
	}

	if ok, _ := regexp.MatchString(`^\w+@\w+[.\w+]+$`, body.Email); !ok {
		return c.JSON(http.StatusBadRequest, api.Return("Invalid E-mail Address", nil))
	}

	// Check authcode
	db, _ := c.Get("db").(*gorm.DB)
	var account Account
	if err := db.Where("email = ?", body.Email).First(&account).Error; err != nil { // not found
		return c.JSON(http.StatusBadRequest, api.Return("E-Mail", echo.Map{"emailok": false}))
	}

	if account.AuthCode != body.AuthCode || time.Now().After(account.AuthCodeExpires) {
		return c.JSON(http.StatusBadRequest, api.Return("AuthCode", echo.Map{"authcodeok": false}))
	}

	if len(body.NewPasswd) < accountPasswdLen {
		return c.JSON(http.StatusBadRequest, api.Return("Invalid Password Length", nil))
	}

	account.Passwd = body.NewPasswd
	account.HashPassword()

	if result := db.Model(&Account{}).Where("id = ?", account.ID).Update("passwd", account.Passwd); result.Error != nil {
		return c.JSON(http.StatusBadRequest, api.Return("DB error", result.Error))
	}

	return c.JSON(http.StatusOK, api.Return("Successfully modified", nil))
}

// @Summary the interface of getting current cookie's info
// @Description
// @Tags Account
// @Produce json
// @Success 200 {string} api.ReturnedData{data=echo.Map{"id": account.ID, "email": account.Email, "type": account.Type, "firstname": account.FirstName, "lastname": account.LastName}}
// @Failure 400 {string} api.ReturnedData{data=nil}
// @Router /account/getinfo [GET]
func (h *AccountHandler) GetInfo(c echo.Context) error {
	cookie, err := c.Cookie("token")
	if err != nil || cookie.Value == "" {
		return c.JSON(http.StatusBadRequest, api.Return("Not logged in", nil))
	}

	db, _ := c.Get("db").(*gorm.DB)
	var account Account
	if err := db.Where("token = ?", cookie.Value).First(&account).Error; err != nil { // not found
		return c.JSON(http.StatusBadRequest, api.Return("Not logged in", nil))
	}
	return c.JSON(http.StatusOK, api.Return("Successfully Get", echo.Map{"id": account.ID, "email": account.Email, "type": account.Type, "firstname": account.FirstName, "lastname": account.LastName}))
}

/**
 * @brief public method for getting current logged-in account's ID.
 */
func getAccountID(c echo.Context) (uint, error) {
	cookie, err := c.Cookie("token")
	if err != nil || cookie.Value == "" {
		return 0, fmt.Errorf("not logged in")
	}

	db, _ := c.Get("db").(*gorm.DB)
	var account Account
	if err := db.Where("token = ?", cookie.Value).First(&account).Error; err != nil { // not found
		return 0, fmt.Errorf("not logged in")
	}
	return account.ID, nil
}

/**
 * @brief middleware for getting current logged-in account's ID.
 */
func CheckAccountID(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		id, err := getAccountID(c)
		if err != nil {
			return c.JSON(403, api.Return("unauthorised", err))
		}
		c.Set("id", id)
		return next(c)
	}
}

/**
 * @brief private method for hashing password
 */
func (u *Account) HashPassword() {
	bytes, _ := bcrypt.GenerateFromPassword([]byte(u.Passwd), bcrypt.DefaultCost)
	u.Passwd = string(bytes)
}

/**
 * @brief private method for generateing cookie token
 */
func (u *Account) GenerateToken() (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"email": u.Email,
	})

	jwtKey := []byte(os.Getenv("JWT_KEY"))
	tokenString, err := token.SignedString(jwtKey)
	return tokenString, err
}
