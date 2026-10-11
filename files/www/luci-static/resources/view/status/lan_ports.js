'use strict';
'require view';
'require fs';
'require poll';

function readPorts() {
 return fs.exec('/usr/libexec/lan-port-status', []).then(function(r) {
  if (r.code !== 0) throw new Error(r.stderr || '读取失败');
  var data = JSON.parse(r.stdout);
  if (!Array.isArray(data.ports) || data.ports.length !== 8)
   throw new Error('端口数据不完整');
  return data;
 });
}

return view.extend({
 load: function() { return readPorts().catch(function(e) { return { error: e.message }; }); },
 render: function(data) {
  var tbody = E('tbody'), stamp = E('p', { 'style': 'color:#666' });
  function draw(d) {
   while (tbody.firstChild) tbody.removeChild(tbody.firstChild);
   function row(label, number, p, duplex) {
    var valid = p && !p.error;
    var speed = valid && p.link ? (p.speed_mbps >= 1000 ? (p.speed_mbps / 1000) + ' Gbps' : p.speed_mbps + ' Mbps') : '—';
    if (valid && p.link && !p.speed_mbps) speed = p.speed_code != null ? '未知（编码 ' + p.speed_code + '）' : '未知';
    tbody.appendChild(E('tr', { 'class': 'tr' }, [
     E('td', { 'class': 'td' }, label),
     E('td', { 'class': 'td' }, number),
     E('td', { 'class': 'td', 'title': p && p.error || '', 'style': 'color:' + (!valid ? '#a66' : p.link ? '#198754' : '#777') }, !valid ? '读取失败' : p.link ? '已连接' : '未连接'),
     E('td', { 'class': 'td', 'style': 'font-weight:600' }, speed),
     E('td', { 'class': 'td' }, valid && p.link ? duplex : '—')
    ]));
   }
   var wan = !d.error && d.wan;
   row('WAN', 'eth0', wan, wan && wan.duplex === 'full' ? '全双工' : wan && wan.duplex === 'half' ? '半双工' : '未知');
   for (var i = 0; i < 8; i++) row('LAN ' + (i + 1), String(7 - i), !d.error && d.ports[7 - i], '—');
   stamp.textContent = d.error ? '无法读取实时状态：' + d.error : '更新时间：' + new Date(d.checked_at).toLocaleTimeString() + ' · 每 5 秒自动刷新';
  }
  draw(data);
  poll.add(function() { return readPorts().then(draw).catch(function(e) { draw({ error: e.message }); }); }, 5);
  return E('div', {}, [
   E('h2', {}, '网口协商速度'),
   E('p', {}, '显示 WAN 和 8 个 RJ45 LAN 口的实际协商速度。LAN1–LAN8 按机壳顺序排列，对应交换机编号 7–0。WAN 双工模式来自物理网口 eth0。'),
   E('table', { 'class': 'table' }, [
    E('thead', {}, [E('tr', { 'class': 'tr table-titles' }, ['端口', '设备 / 芯片编号', '连接状态', '协商速度', '双工模式'].map(function(s) { return E('th', { 'class': 'th' }, s); }))]), tbody
   ]), stamp
  ]);
 },
 handleSave: null,
 handleSaveApply: null,
 handleReset: null
});
