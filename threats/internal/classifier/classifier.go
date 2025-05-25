package classifier

import (
	"fmt"
	"math"
	"reflect"
	"threats/internal/classifier/model"

	onnx "github.com/yalue/onnxruntime_go"
)

type Classifier struct {
}

func NewClassifier() *Classifier {
	err := onnx.InitializeEnvironment()
	if err != nil {
		fmt.Printf("failed to initialize environment: %v", err)
	}

	return &Classifier{}
}

func (c *Classifier) Classify(vector []float32) model.TrafficClass {
	inputShape := onnx.Shape{1, 70}
	inputTensor, err := onnx.NewTensor(inputShape, vector)
	if err != nil {
		fmt.Printf("failed to create input tensor: %v", err)
	}

	// ⚠️ output tensor нужного типа
	outputTensor, err := onnx.NewEmptyTensor[float32](onnx.Shape{1, 2})
	if err != nil {
		fmt.Printf("failed to create output tensor: %v", err)
	}

	inputNames := []string{"float_input"}
	outputNames := []string{"probabilities"}

	session, err := onnx.NewSession(
		"model.onnx",
		inputNames,
		outputNames,
		[]*onnx.Tensor[float32]{inputTensor},
		[]*onnx.Tensor[float32]{outputTensor},
	)
	if err != nil {
		fmt.Printf("failed to create session: %v", err)
	}
	defer session.Destroy()

	if err := session.Run(); err != nil {
		fmt.Printf("failed to run inference: %v", err)
	}

	result := outputTensor.GetData()

	if len(result) < 2 {
		return model.GreenTrafficClass
	}

	greenProba := result[0]
	redProba := result[1]

	if reflect.TypeOf(greenProba).Kind() != reflect.Float32 ||
		reflect.TypeOf(redProba).Kind() != reflect.Float32 ||
		math.IsNaN(float64(greenProba)) ||
		math.IsNaN(float64(redProba)) {
		return model.GreenTrafficClass
	}

	fmt.Println(result)
	var threshold float32 = 0.99
	if greenProba >= threshold {
		return model.GreenTrafficClass
	} else if redProba >= threshold {
		return model.RedTrafficClass
	}

	return model.YellowTrafficClass
}
