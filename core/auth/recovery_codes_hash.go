package auth

func HashRecoveryCode(code string, pepper string) (*PasswordHash, error) {
	return HashPassword(NormalizeRecoveryCode(code), pepper)
}

func VerifyRecoveryCode(code string, pepper string, stored *PasswordHash) (bool, error) {
	return VerifyPassword(NormalizeRecoveryCode(code), pepper, stored)
}

