package util

import "fmt"

// IsLuhnValid проверяет номер по алгоритму Луна
func IsLuhnValid(number string) bool {
	fmt.Printf("=== Debugging Luhn for: %s ===\n", number)

	// Проверка на пустую строку
	if len(number) == 0 {
		fmt.Println("Empty string → invalid")
		return false
	}

	// Проверка на не-цифры
	for _, char := range number {
		if char < '0' || char > '9' {
			fmt.Printf("Non-digit found: %c → invalid\n", char)
			return false
		}
	}
	sum := 0
	isEven := false // с конца, первая цифра не удваивается

	// Начинаем с конца
	for i := len(number) - 1; i >= 0; i-- {
		digit := int(number[i] - '0') // цифра
		fmt.Printf("Position from end: %d | Digit: %d | isEven: %v → ", len(number)-i,
			digit, isEven)

		// Удваиваем перед добавлением сумму
		if isEven {
			doubled := digit * 2
			fmt.Printf("Double: %d → ", doubled)
			if doubled > 9 {
				doubled -= 9
				fmt.Printf("Subtract 9: %d | ", doubled)
			} else {
				fmt.Printf("No subtract | ")
			}
			digit = doubled
		} else {
			fmt.Printf("No double | ")
		}

		// Добавляем digit к sum (вот что ты пропустил!)
		sum += digit
		fmt.Printf("Added: %d | Sum now: %d\n", digit, sum)

		// для следующей цифры
		isEven = !isEven // чередуем
	}

	result := sum%10 == 0
	fmt.Printf("Final sum: %d | %d %% 10 = %d → valid: %v\n\n", sum, sum, sum%10, result)
	return result
}
