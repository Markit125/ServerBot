package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
)

type botAPIResponse struct {
	OK          bool            `json:"ok"`
	Result      json.RawMessage `json:"result,omitempty"`
	Description string          `json:"description,omitempty"`
	ErrorCode   int             `json:"error_code,omitempty"`
}

func (sb *ServerCommanderOverTelegram) sendDocumentStreaming(ctx context.Context, chatID any, fileName string, content io.Reader) error {
	pipeReader, pipeWriter := io.Pipe()
	form := multipart.NewWriter(pipeWriter)

	writeDone := make(chan error, 1)
	go func() {
		defer close(writeDone)

		if err := form.WriteField("chat_id", fmt.Sprint(chatID)); err != nil {
			_ = pipeWriter.CloseWithError(err)
			writeDone <- err
			return
		}
		if err := form.WriteField("caption", fileName); err != nil {
			_ = pipeWriter.CloseWithError(err)
			writeDone <- err
			return
		}

		fileWriter, err := form.CreateFormFile("document", fileName)
		if err != nil {
			_ = pipeWriter.CloseWithError(err)
			writeDone <- err
			return
		}

		if _, err := io.Copy(fileWriter, content); err != nil {
			_ = pipeWriter.CloseWithError(err)
			writeDone <- err
			return
		}
		if err := form.Close(); err != nil {
			_ = pipeWriter.CloseWithError(err)
			writeDone <- err
			return
		}

		writeDone <- pipeWriter.Close()
	}()

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, sb.botAPIURL("sendDocument"), pipeReader)
	if err != nil {
		_ = pipeReader.Close()
		return err
	}
	request.Header.Set("Content-Type", form.FormDataContentType())

	response, err := sb.httpClient.Do(request)
	if err != nil {
		_ = pipeReader.Close()
		return err
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return err
	}

	writeErr := <-writeDone

	var apiResponse botAPIResponse
	if err := json.Unmarshal(responseBody, &apiResponse); err != nil {
		return fmt.Errorf("error decode sendDocument response: status=%d body=%s: %w", response.StatusCode, responseBody, err)
	}
	if !apiResponse.OK {
		return fmt.Errorf("sendDocument failed: status=%d api_code=%d description=%s", response.StatusCode, apiResponse.ErrorCode, apiResponse.Description)
	}
	if writeErr != nil {
		return writeErr
	}

	return nil
}

func (sb *ServerCommanderOverTelegram) sendDocumentLocalPath(ctx context.Context, chatID any, fileName string, filePath string) error {
	values := url.Values{}
	values.Set("chat_id", fmt.Sprint(chatID))
	values.Set("caption", fileName)
	values.Set("document", (&url.URL{Scheme: "file", Path: filePath}).String())

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, sb.botAPIURL("sendDocument"), strings.NewReader(values.Encode()))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	response, err := sb.httpClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return err
	}

	var apiResponse botAPIResponse
	if err := json.Unmarshal(responseBody, &apiResponse); err != nil {
		return fmt.Errorf("error decode sendDocument local-path response: status=%d body=%s: %w", response.StatusCode, responseBody, err)
	}
	if !apiResponse.OK {
		return fmt.Errorf("sendDocument local path failed: status=%d api_code=%d description=%s", response.StatusCode, apiResponse.ErrorCode, apiResponse.Description)
	}

	return nil
}

func (sb *ServerCommanderOverTelegram) botAPIURL(method string) string {
	baseURL := strings.TrimRight(sb.config.BotAPIURL, "/")
	if baseURL == "" {
		baseURL = "https://api.telegram.org"
	}

	return baseURL + "/bot" + sb.config.BotToken + "/" + method
}
