package util

// IsLuhnValid проверяет номер по алгоритму Луна
func IsLuhnValid(number string) bool {

	// Проверка на пустую строку
	if len(number) == 0 {
		return false
	}

	// Проверка на не-цифры
	for _, char := range number {
		if char < '0' || char > '9' {
			return false
		}
	}

	sum := 0
	isEven := false // с конца, первая цифра не удваивается

	// Начинаем с конца
	for i := len(number) - 1; i >= 0; i-- {
		digit := int(number[i] - '0') // цифра

		// Удваиваем перед добавлением сумму
		if isEven {
			doubled := digit * 2
			if doubled > 9 {
				doubled -= 9
			}
			digit = doubled
		}

		// Добавляем digit к sum (вот что ты пропустил!)
		sum += digit

		// для следующей цифры
		isEven = !isEven // чередуем
	}

	return sum%10 == 0
}
