package main

import (
	"math"
	"math/big"
)

func Collatz(n big.Int, reportChannel chan sequenceProgress, reportFrequency int) (report sequenceProgress) {

	steps := 0                        // Number of steps taken to reach 1
	up := 0                           // Number of times the number was multiplied by 3 and added 1
	down := 0                         // Number of times the number was divided by 2
	stonesSlice := make([]big.Int, 0) // Slice to store the stones in the sequence
	maxStone := new(big.Int).Set(&n)  // Maximum stone in the sequence
	number := new(big.Int).Set(&n)    // Original number

	var maxStoneFloat *big.Float
	var stonesScaled []float64
	var stonesF64 []float64
	var stonesString []string
	var upDirection []bool

	stonesSlice = append(stonesSlice, *new(big.Int).Set(&n))
	//loop until n is equal to 1
	for n.Cmp(oneBig) != 0 {
		if new(big.Int).Mod(&n, twoBig).Cmp(zeroBig) == 0 {
			n.Div(&n, twoBig)
			down++
		} else {
			n.Mul(&n, threeBig)
			n.Add(&n, oneBig)
			up++
		}
		steps++

		// If the current stone is greater than the maximum stone, update the maximum stone
		if n.Cmp(maxStone) == 1 {
			maxStone = new(big.Int).Set(&n)
		}

		// Append the current stone to the slice
		stonesSlice = append(stonesSlice, *new(big.Int).Set(&n))

		// If the number of steps is a multiple of the report frequency or the current stone is equal to 1, send a report
		if steps%reportFrequency == 0 || n.Cmp(oneBig) == 0 {

			maxStoneFloat = new(big.Float).SetInt(maxStone)
			stonesScaled = make([]float64, len(stonesSlice))
			stonesF64 = make([]float64, len(stonesSlice))
			stonesString = make([]string, len(stonesSlice))
			upDirection = make([]bool, len(stonesSlice))

			for idx, s := range stonesSlice {

				// Divide each stone by the maximum stone and convert it to a float64
				a, _ := new(big.Float).Quo(new(big.Float).SetInt(&s), maxStoneFloat).Float64()
				stonesScaled[idx] = a

				// Check if the current stone is greater than the previous stone
				if idx > 0 {
					upDirection[idx] = stonesSlice[idx].Cmp(&stonesSlice[idx-1]) == 1
				} else {
					upDirection[idx] = false
				}

				// Convert the stone to a float64
				sf64, err := bigIntToFloat64(&s)

				// If the conversion fails, set the float64 to the maximum float64 value
				if err != nil {
					sf64 = math.MaxFloat64
				}
				// Store the float64 and string representation of the stone
				stonesF64[idx] = sf64
				stonesString[idx] = s.String()
			}
			if reportChannel != nil {

				// Create a new record and send it to the report channel
				report = sequenceProgress{stones: stonesScaled, maxStoneFloat: maxStoneFloat, maxStoneInt: maxStone, lastStone: n.Cmp(oneBig) == 0, stonesRaw: stonesF64, stonesString: stonesString, upMoves: up, downMoves: down, upwards: upDirection, steps: steps, number: number}
				reportChannel <- report
			}
		}
	}

	// Create a new record and return it
	//report = sequenceProgress{maxStoneFloat: maxStoneFloat, maxStoneInt: maxStone, upMoves: up, downMoves: down, steps: steps, maxStoneString: maxStone.String(), number: number, }
	report = sequenceProgress{stones: stonesScaled, maxStoneFloat: maxStoneFloat, maxStoneInt: maxStone, lastStone: n.Cmp(oneBig) == 0, stonesRaw: stonesF64, stonesString: stonesString, upMoves: up, downMoves: down, upwards: upDirection, steps: steps, number: number}

	return
}

func CollatzPerf(n big.Int) (record sequenceProgress) {

	steps := 0
	maxStone := new(big.Int).Set(&n)
	number := new(big.Int).Set(&n)

	for n.Cmp(oneBig) != 0 {

		if new(big.Int).Mod(&n, twoBig).Cmp(zeroBig) == 0 {
			n.Div(&n, twoBig)
		} else {
			n.Mul(&n, threeBig)
			n.Add(&n, oneBig)
		}
		steps++

		if n.Cmp(maxStone) == 1 {
			maxStone = new(big.Int).Set(&n)
		}
	}
	record = sequenceProgress{maxStoneInt: maxStone, steps: steps, maxStoneString: maxStone.String(), number: number}
	return
}
