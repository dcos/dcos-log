package reader

import "testing"

func TestValidateCursor(t *testing.T) {
	validCursors := []string{
		"s=cea8150abb0543deaab113ed2f39b014;i=1;b=2c357020b6e54863a5ac9dee71d5872c;m=33ae8a1;t=53e52ec99a798;x=b3fe26128f768a49",
		"s=cea8150abb0543deaab113ed2f39b014;i=2;b=2c357020b6e54863a5ac9dee71d5872c;m=33ae927;t=53e52ec99a81d;x=b3fe26128f768a49",
		"s=cea8150abb0543deaab113ed2f39b014;i=6;b=2c357020b6e54863a5ac9dee71d5872c;m=33ae985;t=53e52ec99a87b;x=7c043886d4957245",
		"s=cea8150abb0543deaab113ed2f39b014;i=a;b=2c357020b6e54863a5ac9dee71d5872c;m=33ae9af;t=53e52ec99a8a6;x=b7899e663a8cd564",
	}

	invalidCursors := []string{
		"s=XXcea8150abb0543deaab113ed2f39b014;i=c;b=2c357020b6e54863a5ac9dee71d5872c;m=33ae9bc;t=53e52ec99a8b3;x=512d8e1b6a2c9693",
		"p=cea8150abb0543deaab113ed2f39b014;i=c;b=2c357020b6e54863a5ac9dee71d5872c;m=33ae9bc;t=53e52ec99a8b3;x=512d8e1b6a2c9693",
		"s=cea8150abb0543deaab113ed2f39b014;i=ffffffffffffffff1;b=2c357020b6e54863a5ac9dee71d5872c;m=33ae9bc;t=53e52ec99a8b3;x=512d8e1b6a2c9693",
		"s=cea8150abb0543deaab113ed2f39b014;o=a;b=2c357020b6e54863a5ac9dee71d5872c;m=33ae9af;t=53e52ec99a8a6;x=b7899e663a8cd564",
		"s=cea8150abb0543deaab113ed2f39b014;i=a;p=2c357020b6e54863a5ac9dee71d5872c;m=33ae9af;t=53e52ec99a8a6;x=b7899e663a8cd564",
		"s=cea8150abb0543deaab113ed2f39b014;i=a;b=2c357020b6e54863a5ac9dee71d5872c;l=33ae9af;t=53e52ec99a8a6;x=b7899e663a8cd564",
		"s=cea8150abb0543deaab113ed2f39b014;i=a;b=2c357020b6e54863a5ac9dee71d5872c;m=33ae9af;a=53e52ec99a8a6;x=b7899e663a8cd564",
		"s=cea8150abb0543deaab113ed2f39b014;i=a;b=2c357020b6e54863a5ac9dee71d5872c;m=33ae9af;t=53e52ec99a8a6;i=b7899e663a8cd564",
		"s=cea8150abb0543deaab113ed2f39b014;i=a;b=2c357020b6e54863a5ac9dee71d5872c;m=33ae9af;t=53e52ec99a8a6;x=V7899e663a8cd564",
	}

	for _, validCursor := range validCursors {
		if err := validateCursor(validCursor); err != nil {
			t.Fatalf("Cursor %s is valid, but did not pass validation", validCursor)
		}
	}

	for _, invalidCursor := range invalidCursors {
		if err := validateCursor(invalidCursor); err == nil {
			t.Fatalf("Cursor %s must be invalid, but it was validated", invalidCursor)
		}
	}
}
