package classifier

import (
	"log"
	"threats/internal/classifier/model"

	onnx "github.com/yalue/onnxruntime_go"
)

type Classifier struct {
}

func NewClassifier() *Classifier {
	err := onnx.InitializeEnvironment()
	if err != nil {
		log.Fatalf("failed to initialize environment: %v", err)
	}

	return &Classifier{}
}

func (c *Classifier) Classify(vector []float32) model.TrafficClass {
	inputShape := onnx.Shape{1, 78}
	inputTensor, err := onnx.NewTensor(inputShape, vector)
	if err != nil {
		log.Fatalf("failed to create input tensor: %v", err)
	}

	// ⚠️ output tensor нужного типа
	outputTensor, err := onnx.NewEmptyTensor[float32](onnx.Shape{1, 2})
	if err != nil {
		log.Fatalf("failed to create output tensor: %v", err)
	}

	inputNames := []string{"float_input"}
	outputNames := []string{"output_label"}

	session, err := onnx.NewSession(
		"model.onnx",
		inputNames,
		outputNames,
		[]*onnx.Tensor[float32]{inputTensor},
		[]*onnx.Tensor[float32]{outputTensor},
	)
	if err != nil {
		log.Fatalf("failed to create session: %v", err)
	}
	defer session.Destroy()

	if err := session.Run(); err != nil {
		log.Fatalf("failed to run inference: %v", err)
	}

	result := outputTensor.GetData()

	if len(result) == 0 {
		log.Fatalf("no output from model")
	}

	if result[0] == 1 {
		return model.RedTrafficClass
	}
	return model.GreenTrafficClass
}
