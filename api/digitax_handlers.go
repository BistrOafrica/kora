package api

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/asenawritescode/kora/secret"
	"github.com/gin-gonic/gin"
)

// HandleDigiTaxWebhook receives DigiTax's asynchronous sale.sync callback.
// It is intentionally registered on the public route group: DigiTax cannot
// maintain a Kora session or CSRF token. Authentication is done with the
// site secret digitax_webhook_secret in either the X-DigiTax-Webhook-Secret
// header or the token query parameter.
func (h *Handler) HandleDigiTaxWebhook(c *gin.Context) {
	site := c.GetString("site_name")
	db := h.queryDB(c)
	if site == "" || db == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "site database unavailable"})
		return
	}

	expected, err := secret.NewStore(db).Get(site, "digitax_webhook_secret")
	if err != nil || expected == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "DigiTax webhook secret is not configured"})
		return
	}
	provided := c.GetHeader("X-DigiTax-Webhook-Secret")
	if provided == "" {
		provided = c.Query("token")
	}
	if subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) != 1 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid webhook secret"})
		return
	}

	raw, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid webhook body"})
		return
	}
	var envelope struct {
		Data  map[string]any `json:"data"`
		Event string         `json:"event"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil || envelope.Event == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid DigiTax webhook body"})
		return
	}
	if envelope.Event != "sale.sync" {
		// Acknowledge other valid DigiTax events without mutating an invoice.
		c.JSON(http.StatusOK, gin.H{"status": "ignored", "event": envelope.Event})
		return
	}

	traderInvoice := stringValue(envelope.Data["trader_invoice_number"])
	if traderInvoice == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "trader_invoice_number is required"})
		return
	}

	queueStatus := strings.ToLower(stringValue(envelope.Data["queue_status"]))
	status := mapDigiTaxStatus(queueStatus)
	signedAt := parseDigiTaxTime(stringValue(envelope.Data["date"]), stringValue(envelope.Data["time"]))
	var existing int
	if err := db.QueryRow("SELECT COUNT(*) FROM `tabeTIMS Invoice` WHERE trader_invoice_number = ?", traderInvoice).Scan(&existing); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to find eTIMS invoice"})
		return
	}
	if existing == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "eTIMS invoice not found", "trader_invoice_number": traderInvoice})
		return
	}

	_, err = db.Exec(`UPDATE `+"`tabeTIMS Invoice`"+` SET
		provider = ?, trader_invoice_number = ?, provider_status = ?, status = ?,
		provider_sale_id = ?, etims_reference = ?, invoice_number = ?, receipt_number = ?,
		serial_number = ?, internal_data = ?, receipt_signature = ?, etims_url = ?,
		sale_detail_url = ?, callback_event = ?, webhook_received_at = ?, signed_at = ?,
		customer_pin = ?, tax_summary = ?, submission_response = ?
		WHERE trader_invoice_number = ?`,
		"DigiTax", traderInvoice, queueStatus, status,
		stringValue(envelope.Data["digitax_id"]), stringValue(envelope.Data["serial_number"]),
		stringValue(envelope.Data["invoice_number"]), stringValue(envelope.Data["receipt_number"]),
		stringValue(envelope.Data["serial_number"]), stringValue(envelope.Data["internal_data"]),
		stringValue(envelope.Data["receipt_signature"]), stringValue(envelope.Data["etims_url"]),
		stringValue(envelope.Data["sale_detail_url"]), envelope.Event, time.Now(), signedAt,
		stringValue(envelope.Data["customer_pin"]), jsonValue(envelope.Data["sales_tax_summary"]), string(raw),
		traderInvoice)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to record DigiTax callback"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "received", "event": envelope.Event, "trader_invoice_number": traderInvoice})
}

func stringValue(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func jsonValue(v any) any {
	if v == nil {
		return nil
	}
	b, _ := json.Marshal(v)
	return string(b)
}

func mapDigiTaxStatus(queueStatus string) string {
	switch queueStatus {
	case "completed":
		return "Accepted"
	case "failed":
		return "Failed"
	case "submitted":
		return "Submitted"
	case "pending":
		return "Pending"
	default:
		return "Pending"
	}
}

func parseDigiTaxTime(date, clock string) any {
	if date == "" || clock == "" {
		return nil
	}
	for _, layout := range []string{"02/01/2006 03:04:05 pm", "02/01/2006 03:04:05 PM"} {
		if parsed, err := time.ParseInLocation(layout, date+" "+clock, time.Local); err == nil {
			return parsed
		}
	}
	return nil
}
