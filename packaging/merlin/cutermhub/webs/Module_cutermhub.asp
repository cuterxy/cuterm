<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.0 Transitional//EN" "http://www.w3.org/TR/xhtml1/DTD/xhtml1-transitional.dtd">
<html xmlns="http://www.w3.org/1999/xhtml">
<html xmlns:v>
<head>
<meta http-equiv="X-UA-Compatible" content="IE=Edge"/>
<meta http-equiv="Content-Type" content="text/html; charset=utf-8" />
<meta HTTP-EQUIV="Pragma" CONTENT="no-cache">
<meta HTTP-EQUIV="Expires" CONTENT="-1">
<link rel="shortcut icon" href="images/favicon.png">
<link rel="icon" href="images/favicon.png">
<title>cuterm-hub</title>
<link rel="stylesheet" type="text/css" href="index_style.css"/>
<link rel="stylesheet" type="text/css" href="form_style.css"/>
<link rel="stylesheet" type="text/css" href="usp_style.css"/>
<link rel="stylesheet" type="text/css" href="css/element.css">
<link rel="stylesheet" type="text/css" href="res/softcenter.css">
<script language="JavaScript" type="text/javascript" src="/js/jquery.js"></script>
<script language="JavaScript" type="text/javascript" src="/state.js"></script>
<script language="JavaScript" type="text/javascript" src="/popup.js"></script>
<script language="JavaScript" type="text/javascript" src="/help.js"></script>
<script language="JavaScript" type="text/javascript" src="/general.js"></script>
<script language="JavaScript" type="text/javascript" src="/res/softcenter.js"></script>
<style>
	.ks_btn {
		border: 1px solid #222;
		font-size:10pt;
		color: #fff;
		padding: 5px 5px 5px 5px;
		border-radius: 5px 5px 5px 5px;
		width:14%;
		background: linear-gradient(to bottom, #003333  0%, #000000 100%);
		background: linear-gradient(to bottom, #91071f  0%, #700618 100%); /* W3C rogcss */
	}
	.ks_btn:hover, {
		border: 1px solid #222;
		font-size:10pt;
		color: #fff;
		padding: 5px 5px 5px 5px;
		border-radius: 5px 5px 5px 5px;
		width:14%;
		background: linear-gradient(to bottom, #27c9c9  0%, #279fd9 100%);
		background: linear-gradient(to bottom, #cf0a2c  0%, #91071f 100%); /* W3C rogcss */
	}
	#cutermhub_switch, #hub_panel { border:1px solid #222; border:1px solid #91071f; } /* W3C rogcss */
	#hub_frame {
		width: 100%;
		height: 640px;
		border: none;
		background: #fff;
	}
</style>
<script>
var dbus = {};
var hub_port = "7682";

function E(id) { return document.getElementById(id); }

function init() {
	show_menu(menu_hook);
	get_dbus_data();
	check_status();
}

function get_dbus_data() {
	$.ajax({
		type: "GET",
		url: "/_api/cutermhub",
		dataType: "json",
		cache: false,
		async: false,
		success: function(data) {
			dbus = data.result[0];
			conf2obj();
		},
		error: function(XmlHttpRequest, textStatus, errorThrown){
			console.log(XmlHttpRequest.responseText);
			alert("skipd数据读取错误，请用在chrome浏览器中按F12键后，在console页面获取错误信息！");
		}
	});
}

function conf2obj(){
	E("cutermhub_enable").checked = dbus["cutermhub_enable"] == "1";
	if (dbus["cutermhub_version"])
		E("cutermhub_version").innerHTML = "当前版本：" + dbus["cutermhub_version"];
	if (dbus["cutermhub_port"])
		hub_port = dbus["cutermhub_port"];
}

function hub_url() {
	return "http://" + location.hostname + ":" + hub_port;
}

function check_status() {
	// 直接用图片探测 hub 管理页是否可达，绕过跨域限制
	var img = new Image();
	img.onload = function() { set_status(true); };
	img.onerror = function() { set_status(false); };
	img.src = hub_url() + "/icon-64.png?_t=" + new Date().getTime();
	setTimeout("check_status();", 5000);
}

function set_status(running) {
	if (running) {
		E("run_status").innerHTML = "<span style='color:#1bbf16'>运行中</span>";
		E("hub_addr").innerHTML = '<a href="' + hub_url() + '" target="_blank" style="color:#00ffe4;text-decoration:underline;"><em>' + hub_url() + '</em></a>';
		if (E("hub_frame").getAttribute("src") != hub_url() + "/")
			E("hub_frame").src = hub_url() + "/";
		E("hub_panel").style.display = "";
	} else {
		E("run_status").innerHTML = "未运行";
		E("hub_addr").innerHTML = "-";
		E("hub_panel").style.display = "none";
	}
}

function save(flag) {
	var dbus_new = {};
	dbus_new["cutermhub_enable"] = E("cutermhub_enable").checked ? '1' : '0';
	var id = parseInt(Math.random() * 100000000);
	var postData = {"id": id, "method": "cutermhub_config.sh", "params": [flag], "fields": dbus_new };
	$.ajax({
		url: "/_api/",
		cache: false,
		async: false,
		type: "POST",
		dataType: "json",
		data: JSON.stringify(postData),
		success: function(response) {
			if (response.result == id){
				setTimeout("refreshpage();", 1500);
			}
		}
	});
}

function menu_hook() {
	tabtitle[tabtitle.length - 1] = new Array("", "cuterm-hub");
	tablink[tablink.length - 1] = new Array("", "Module_cutermhub.asp");
}

function reload_Soft_Center(){
	location.href = "/Module_Softcenter.asp";
}
</script>
</head>
<body onload="init();">
<div id="TopBanner"></div>
<div id="Loading" class="popup_bg"></div>
<table class="content" align="center" cellpadding="0" cellspacing="0">
	<tr>
		<td width="17">&nbsp;</td>
		<td valign="top" width="202">
			<div id="mainMenu"></div>
			<div id="subMenu"></div>
		</td>
		<td valign="top">
			<div id="tabMenu" class="submenuBlock"></div>
			<table width="98%" border="0" align="left" cellpadding="0" cellspacing="0" style="display: block;">
				<tr>
					<td align="left" valign="top">
						<div>
							<table width="760px" border="0" cellpadding="5" cellspacing="0" bordercolor="#6b8fa3" class="FormTitle" id="FormTitle">
								<tr>
									<td bgcolor="#4D595D" colspan="3" valign="top">
										<div>&nbsp;</div>
										<div style="float:left;" class="formfonttitle" style="padding-top: 12px">cuterm-hub</div>
										<div style="float:right; width:15px; height:25px;margin-top:10px"><img id="return_btn" onclick="reload_Soft_Center();" align="right" style="cursor:pointer;position:absolute;margin-left:-30px;margin-top:-25px;" title="返回软件中心" src="/images/backprev.png" onMouseOver="this.src='/images/backprevclick.png'" onMouseOut="this.src='/images/backprev.png'"></img></div>
										<div style="margin:30px 0 10px 5px;" class="splitLine"></div>
										<div style="margin-left:5px;" id="head_illustrate">
											<li><em>cuterm-hub</em> 是 cuterm 的集群管理工具：连接局域网内任意多台 cuterm 节点，在一个页面上创建、接入、重命名、关闭所有节点的终端。</li>
											<li><font color="#ffcc00">注意：cuterm-hub 没有鉴权，任何能访问管理页面的设备都能操作节点上的 shell，请仅在可信内网中使用，不要对公网开放端口！</font></li>
										</div>
										<div id="cutermhub_switch" style="margin:5px 0px 0px 0px;">
											<table width="100%" border="1" align="center" cellpadding="4" cellspacing="0" bordercolor="#6b8fa3" class="FormTable">
												<thead>
												<tr>
													<td colspan="2">cuterm-hub - 开关/状态</td>
												</tr>
												</thead>
												<tr id="switch_tr">
													<th>
														<label>开启cuterm-hub</label>
													</th>
													<td colspan="2">
														<div class="switch_field" style="display:table-cell">
															<label for="cutermhub_enable">
																<input id="cutermhub_enable" class="switch" type="checkbox" style="display: none;">
																<div class="switch_container" >
																	<div class="switch_bar"></div>
																	<div class="switch_circle transition_style">
																		<div></div>
																	</div>
																</div>
															</label>
														</div>
														<div style="display:table-cell;float: left;margin-left:270px;margin-top:-32px;position: absolute;padding: 5.5px 0px;">
															<a type="button" class="ks_btn" target="_blank" href="https://github.com/cuterxy/cuterm">项目主页</a>
														</div>
														<div id="cutermhub_version" style="padding-top:5px;margin-right:50px;margin-top:-30px;float: right;"></div>
													</td>
												</tr>
												<tr id="status_tr">
													<th width="35%">运行状态</th>
													<td><span id="run_status">检测中...</span></td>
												</tr>
												<tr id="addr_tr">
													<th width="35%">管理地址</th>
													<td><span id="hub_addr">-</span></td>
												</tr>
											</table>
										</div>
										<div id="hub_panel" style="display: none;margin-top:10px;">
											<iframe id="hub_frame" src="about:blank"></iframe>
										</div>
										<div id="apply_button" class="apply_gen">
											<input id="apply_button-1" class="button_gen" type="button" onclick="save(1)" value="提交">
										</div>
										<div style="margin-left:5px;" id="note_illustrate">
											<li>开启后，管理页面监听 <em>7682</em> 端口（可在 cuterm-hub 设置页修改），局域网设备均可直接访问。</li>
											<li>在管理页面的设置页中添加节点（名称 + host:port），节点运行任意平台的原版 cuterm 即可，无需额外插件。</li>
											<li>插件配置保存在 <em>/koolshare/configs/cutermhub</em>，重启路由器不丢失；卸载插件不会删除该目录。</li>
										</div>
										<div class="KoolshareBottom" style="margin-top:50px;">
											GitHub: <a href="https://github.com/cuterxy/cuterm" target="_blank"><i><u>https://github.com/cuterxy/cuterm</u></i></a><br />
										</div>
									</td>
								</tr>
							</table>
						</div>
					</td>
				</tr>
			</table>
		</td>
		<td width="10" align="center" valign="top"></td>
	</tr>
</table>
<div id="footer"></div>
</body>
</html>
