var Ehr = Ehr || {};

// Mock variables for Demo environment
var kqToken = false;
var qtllq1 = "1";
var authPhis = "demo-auth";
var Login = { fullName: "DemoUser", domainId: "demo-org", id: "demo-id" };
var CommonUtil = {
    templateApply: function(url, params) {
        return url + "?user=" + params.userName;
    }
};
Ehr.urls = { phisIndex: "/phis/index" };

Ehr.phisIndex = function() {
//	if(Ehr.isChronicDisease){
	if(kqToken){
			var fn=function(data){
				var jsonObj = eval("(" + data + ")");
				if(jsonObj.code==0&&jsonObj.result){
					if(qtllq1=="1"){
						Ehr.openChrome(CommonUtil.templateApply(Ehr.urls.phisIndexToken,{authPhis:authPhis,userName:encodeURI(Login.fullName),orgCode:Login.domainId,toKen:jsonObj.result}));
					}else{
						openObject.open(CommonUtil.templateApply(Ehr.urls.phisIndexToken,{authPhis:authPhis,userName:encodeURI(Login.fullName),orgCode:Login.domainId,toKen:jsonObj.result}),"800","1400");
					}
				}else{
					alert("获取toKen失败")
				}
			}
		jx.load(CommonUtil.templateApply(Ehr.urls.getToKenPhis,{authPhis:authPhis}),fn,"text","get");
	}else{
		if(qtllq1=="1"){
			Ehr.openChrome(CommonUtil.templateApply(Ehr.urls.phisIndex,{authPhis:authPhis,userName:encodeURI(Login.fullName),orgCode:Login.domainId,userId:Login.id}));
		}else{
//			window.showModalDialog(CommonUtil.templateApply(Ehr.urls.phisIndex,{authPhis:authPhis,userName:encodeURI(Login.fullName),orgCode:Login.domainId}),null,"dialogHeight:600px;dialogWidth:1300px;center:1;status:no;");
			openObject.open(CommonUtil.templateApply(Ehr.urls.phisIndex,{authPhis:authPhis,userName:encodeURI(Login.fullName),orgCode:Login.domainId,userId:Login.id}),"800","1400");
		}
	}
}

Ehr.openChrome = function(url){
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
	}else{
		window.open(url,"", "height=600, width=1200, top=10, left=150, toolbar=y, menubar=no, scrollbars=yes, resizable=no,location=no, status=no");
	}
}