package weft

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/loreste/weft/internal/llmpack"
)

// WritePrompt writes the LLM system card (+ optional few-shot) to w.
func WritePrompt(w io.Writer, fewShot int) error {
	if _, err := io.WriteString(w, llmpack.SystemCard()); err != nil {
		return err
	}
	if fewShot > 0 {
		if _, err := io.WriteString(w, "\n"); err != nil {
			return err
		}
		if _, err := io.WriteString(w, llmpack.FewShot(fewShot)); err != nil {
			return err
		}
	}
	return nil
}

// WriteTrainJSONL writes the supervised corpus to w.
func WriteTrainJSONL(w io.Writer) error {
	_, err := io.WriteString(w, llmpack.TrainJSONL())
	if !strings.HasSuffix(llmpack.TrainJSONL(), "\n") && err == nil {
		_, err = io.WriteString(w, "\n")
	}
	return err
}

// WriteTrainChatJSONL writes OpenAI chat-format JSONL for fine-tuning.
func WriteTrainChatJSONL(w io.Writer) error {
	enc := json.NewEncoder(w)
	for _, e := range llmpack.Examples() {
		row := map[string]any{
			"messages": llmpack.ChatMessages(e),
		}
		if err := enc.Encode(row); err != nil {
			return err
		}
	}
	return nil
}

// ValidateTrainCorpus ensures all training outputs are valid Weft.
func ValidateTrainCorpus() error {
	errs := llmpack.ValidateAll()
	if len(errs) == 0 {
		fmt.Fprintf(os.Stdout, "ok — %d training examples valid\n", len(llmpack.Examples()))
		return nil
	}
	for _, e := range errs {
		fmt.Fprintln(os.Stderr, e)
	}
	return fmt.Errorf("%d invalid examples", len(errs))
}
