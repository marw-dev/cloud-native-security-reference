package auth

import (
	"bytes"
	"encoding/base32"
	"fmt"
	"image/png"
	"log/slog"
	"time"

	"github.com/boombuler/barcode"
	"github.com/boombuler/barcode/qr"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

func GenerateOTPSecret() (*otp.Key, error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "Athena", // TODO: Aus Konfiguration laden
		AccountName: "",
		Period:      30,
		Digits:      otp.DigitsSix,
		Algorithm:   otp.AlgorithmSHA1,
	})
	if err != nil {
		slog.Error("Fehler beim Generieren des OTP-Secrets", slog.Any("error", err))
		return nil, fmt.Errorf("fehler beim Generieren des OTP-Secrets: %w", err)
	}
	return key, nil
}

func GenerateOTPAuthURL(key *otp.Key) string {
	return key.URL()
}

func GenerateQRCodePNG(otpAuthURL string) ([]byte, error) {
	qrCode, err := qr.Encode(otpAuthURL, qr.M, qr.Auto)
	if err != nil {
		slog.Error("Fehler beim Erstellen des QR-Codes", slog.Any("error", err), slog.String("url", otpAuthURL))
		return nil, fmt.Errorf("fehler beim Erstellen des QR-Codes: %w", err)
	}

	qrCodeScaled, err := barcode.Scale(qrCode, 200, 200)
	if err != nil {
		slog.Error("Fehler beim Skalieren des QR-Codes", slog.Any("error", err))
		return nil, fmt.Errorf("fehler beim Skalieren des QR-Codes: %w", err)
	}

	buffer := new(bytes.Buffer)
	err = png.Encode(buffer, qrCodeScaled)
	if err != nil {
		slog.Error("Fehler beim Encodieren des QR-Codes als PNG", slog.Any("error", err))
		return nil, fmt.Errorf("fehler beim Encodieren des QR-Codes als PNG: %w", err)
	}

	return buffer.Bytes(), nil
}

func ValidateOTPCode(secretBase32 string, userCode string) (bool, error) {
	_, err := base32.StdEncoding.DecodeString(secretBase32)
	if err != nil {
		slog.Error("Gespeichertes OTP-Secret ist ungültig Base32 kodiert", slog.Any("error", err))
		return false, fmt.Errorf("interner Fehler bei OTP-Prüfung")
	}

	valid, err := totp.ValidateCustom(userCode, secretBase32, time.Now().UTC(), totp.ValidateOpts{
		Period:    30,
		Skew:      1,
		Digits:    otp.DigitsSix,
		Algorithm: otp.AlgorithmSHA1,
	})

	if err != nil {
		slog.Warn("Fehler während der OTP-Validierung (nicht unbedingt falscher Code)", slog.Any("error", err))
		return false, nil
	}

	return valid, nil
}