package weather

import (
	"testing"

	asserts "github.com/stretchr/testify/assert"
)

func TestCalculateAQI(t *testing.T) {
	assert := asserts.New(t)
	assert.Equal(0, computeAQI(-0.1))
	assert.Equal(0, computeAQI(0.0))
	assert.Equal(25, computeAQI(6.0))
	assert.Equal(25, computeAQI(6.1))
	assert.Equal(26, computeAQI(6.2))
	assert.Equal(50, computeAQI(12.0))
	assert.Equal(51, computeAQI(12.1))
	assert.Equal(100, computeAQI(35.4))
	assert.Equal(101, computeAQI(35.5))
	assert.Equal(150, computeAQI(55.4))
	assert.Equal(151, computeAQI(55.5))
	assert.Equal(200, computeAQI(150.4))
	assert.Equal(201, computeAQI(150.5))
	assert.Equal(300, computeAQI(250.4))
	assert.Equal(301, computeAQI(250.5))
	assert.Equal(400, computeAQI(350.4))
	assert.Equal(401, computeAQI(350.5))
	assert.Equal(500, computeAQI(500.4))
	assert.Equal(500, computeAQI(600.5))
}
