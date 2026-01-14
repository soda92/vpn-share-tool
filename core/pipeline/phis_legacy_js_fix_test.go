package pipeline

import (
	"strings"
	"testing"

	"github.com/soda92/vpn-share-tool/core/models"
)

func TestFixLegacyJS(t *testing.T) {
	// Content from ehr.js
	input := `Ehr.openChrome = function(url){
	if(navigator.appVersion.indexOf("MSIE")>=0){
		 var executableFullPath = "D:\\pb\\chromerun.exe "+url; 
		    try
		    {
		        var shellActiveXObject = new ActiveXObject("WScript.Shell");
		        if ( !shellActiveXObject )
		        {
		            alert('Could not get reference to WScript.Shell');
		            return;
		        }

		        shellActiveXObject.Run(executableFullPath, 1, false);
		        shellActiveXObject = null;
		    }
		    catch (errorObject)
		    {
		        alert('Error:\n' + errorObject.error);
		    }
	}
	else{
		window.open(url, "", "height=600, width=1200, top=10, left=150, toolbar=y, menubar=no, scrollbars=yes, resizable=no,location=no, status=no");
	}
}`

	// Create a dummy context
	ctx := &models.ProcessingContext{}

	output := FixLegacyJS(ctx, input)

	// Check if Ehr.openChrome was modified to return early
	expectedStart := `Ehr.openChrome = function(url){ window.open(url, "_blank"); return;`
	if !strings.Contains(output, expectedStart) {
		t.Errorf("FixLegacyJS failed to replace Ehr.openChrome function signature.\nExpected to contain: %s\nGot:\n%s", expectedStart, output)
	}

	// Check if the original complex window.open was also replaced (redundant check if return is there, but good for completeness)
	// The original regex replaces the window.open call in the else block.
	// However, if the first regex matches and inserts 'return', the rest of the code is effectively dead but still present unless we remove it.
	// Wait, the current implementation of FixLegacyJS does REPLACE the function signature line.
	// But it also runs `reEhrWindowOpen.ReplaceAllString`.

	// Let's verify if `reEhrWindowOpen` also matched and replaced the specific window.open call later in the string.
	// In the original input: `window.open(url,"", ...)` is present.
	// In the output, it should be replaced by `window.open(url, "_blank");`.

	expectedWindowOpen := `window.open(url, "_blank");`
	// The output should contain this string at least once (inserted at the top)
	// and potentially twice if the one in the `else` block was also replaced.

	if !strings.Contains(output, expectedWindowOpen) {
		t.Errorf("FixLegacyJS failed to inject window.open replacement.")
	}

	// Check that the modal check replacement works (another case)
	inputModal := `if(window.showModalDialog == undefined)`
	outputModal := FixLegacyJS(ctx, inputModal)
	if !strings.Contains(outputModal, "if(true)") {
		t.Errorf("FixLegacyJS failed to replace showModalDialog check.")
	}
}
