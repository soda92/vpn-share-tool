package pipeline


type SystemDefinition struct {
	ID         string
	Name       string
	ProbeURLs  []string
	Processors []ContentProcessor
}

// We need to move ContentProcessor definition to models or keep it here and import models?
// ContentProcessor is defined in pipeline.go. It depends on models.ProcessingContext.
// SystemDefinition needs ContentProcessor.

// Let's rely on string IDs for processors to avoid circular imports if we define Systems in models?
// No, Systems should be defined in pipeline package.

var DefinedSystems = []SystemDefinition{
	{
		ID:        "HIS",
		Name:      "Legacy HIS",
		ProbeURLs: []string{"cis/images/img/LOGO-HIS-LOGIN.png"},
		Processors: []ContentProcessor{
			FixLegacyJS,
		},
	},
	{
		ID:        "PHIS",
		Name:      "Public Health",
		ProbeURLs: []string{"phis/static/images/logins/bg-denglu.png"},
		Processors: []ContentProcessor{
			RewritePhisURLs,
			InjectCaptchaSolver,
		},
	},
}
