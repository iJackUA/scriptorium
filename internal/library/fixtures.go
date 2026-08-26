package library

// Fixture returns a hardcoded Library. It stands in for the workspace loader
// until Books and Series are read from disk, so the UI has something to render.
func Fixture() Library {
	return Library{Series: []Series{
		{
			Code:           "holmes",
			Name:           "The Adventures of Sherlock Holmes",
			SourceLanguage: "English",
			Books: []Book{
				{
					Code:   "adventures",
					Title:  "The Adventures of Sherlock Holmes",
					Author: "Arthur Conan Doyle",
					Targets: []TranslationTarget{
						{Language: "Ukrainian", Status: StatusTranslated},
						{Language: "German", Status: StatusTranslating},
					},
				},
				{
					Code:   "memoirs",
					Title:  "The Memoirs of Sherlock Holmes",
					Author: "Arthur Conan Doyle",
					Targets: []TranslationTarget{
						{Language: "Ukrainian", Status: StatusDictionaryReady},
					},
				},
				{
					Code:    "return",
					Title:   "The Return of Sherlock Holmes",
					Author:  "Arthur Conan Doyle",
					Targets: []TranslationTarget{{Language: "Ukrainian", Status: StatusNew}},
				},
			},
		},
		{
			Code:           "solaris",
			Name:           "Solaris",
			SourceLanguage: "Polish",
			Books: []Book{
				{
					Code:   "solaris",
					Title:  "Solaris",
					Author: "Stanisław Lem",
					Targets: []TranslationTarget{
						{Language: "Ukrainian", Status: StatusAnalyzing},
						{Language: "English", Status: StatusFailed},
					},
				},
			},
		},
	}}
}
