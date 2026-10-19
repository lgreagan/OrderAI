/*
 * 订单字段中英文对照。
 * 国家与运输方式在录入时是自由文本，历史数据多为英文；
 * 截图 / 打印时统一转成中文显示，已经是中文的原样返回。
 *
 * 字典分两层：
 *   1. 内置（下面的 DEFAULT_*）——常用国家与运输方式，兜底用；
 *   2. 维护（系统设置 → 翻译字典）——存在 app_settings，运行时从 /api/i18n 拉取，
 *      同名键覆盖内置值，因此新增客户国别、新运输方式只要在设置页加一行，不用改代码。
 * 页面渲染前 await OrderI18N.ready 即可保证已经应用维护过的字典。
 */
(function () {
    var DEFAULT_COUNTRY = {
        'UK': '英国', 'GB': '英国', 'UNITED KINGDOM': '英国', 'ENGLAND': '英国', 'BRITAIN': '英国',
        'GERMANY': '德国', 'DE': '德国', 'DEUTSCHLAND': '德国',
        'USA': '美国', 'US': '美国', 'UNITED STATES': '美国', 'UNITED STATES OF AMERICA': '美国', 'AMERICA': '美国',
        'CANADA': '加拿大', 'CA': '加拿大',
        'MEXICO': '墨西哥', 'MX': '墨西哥',
        'BRAZIL': '巴西', 'BR': '巴西',
        'FRANCE': '法国', 'FR': '法国',
        'ITALY': '意大利', 'IT': '意大利',
        'SPAIN': '西班牙', 'ES': '西班牙',
        'PORTUGAL': '葡萄牙', 'PT': '葡萄牙',
        'NETHERLANDS': '荷兰', 'NL': '荷兰', 'HOLLAND': '荷兰',
        'BELGIUM': '比利时', 'BE': '比利时',
        'LUXEMBOURG': '卢森堡',
        'SWITZERLAND': '瑞士', 'CH': '瑞士',
        'AUSTRIA': '奥地利', 'AT': '奥地利',
        'POLAND': '波兰', 'PL': '波兰',
        'CZECH REPUBLIC': '捷克', 'CZECH': '捷克', 'CZ': '捷克',
        'SLOVAKIA': '斯洛伐克',
        'HUNGARY': '匈牙利', 'HU': '匈牙利',
        'ROMANIA': '罗马尼亚', 'RO': '罗马尼亚',
        'BULGARIA': '保加利亚',
        'GREECE': '希腊', 'GR': '希腊',
        'SWEDEN': '瑞典', 'SE': '瑞典',
        'NORWAY': '挪威', 'NO': '挪威',
        'DENMARK': '丹麦', 'DK': '丹麦',
        'FINLAND': '芬兰', 'FI': '芬兰',
        'IRELAND': '爱尔兰', 'IE': '爱尔兰',
        'ICELAND': '冰岛',
        'RUSSIA': '俄罗斯', 'RU': '俄罗斯',
        'UKRAINE': '乌克兰',
        'TURKEY': '土耳其', 'TR': '土耳其',
        'ISRAEL': '以色列', 'IL': '以色列',
        'UAE': '阿联酋', 'UNITED ARAB EMIRATES': '阿联酋',
        'SAUDI ARABIA': '沙特阿拉伯', 'SA': '沙特阿拉伯',
        'QATAR': '卡塔尔',
        'KUWAIT': '科威特',
        'EGYPT': '埃及',
        'SOUTH AFRICA': '南非', 'ZA': '南非',
        'NIGERIA': '尼日利亚',
        'KENYA': '肯尼亚',
        'MOROCCO': '摩洛哥',
        'AUSTRALIA': '澳大利亚', 'AU': '澳大利亚',
        'NEW ZEALAND': '新西兰', 'NZ': '新西兰',
        'JAPAN': '日本', 'JP': '日本',
        'SOUTH KOREA': '韩国', 'KOREA': '韩国', 'KR': '韩国',
        'SINGAPORE': '新加坡', 'SG': '新加坡',
        'MALAYSIA': '马来西亚', 'MY': '马来西亚',
        'THAILAND': '泰国', 'TH': '泰国',
        'VIETNAM': '越南', 'VN': '越南',
        'PHILIPPINES': '菲律宾', 'PH': '菲律宾',
        'INDONESIA': '印度尼西亚', 'ID': '印度尼西亚',
        'INDIA': '印度', 'IN': '印度',
        'PAKISTAN': '巴基斯坦',
        'BANGLADESH': '孟加拉国',
        'SRI LANKA': '斯里兰卡',
        'CHINA': '中国', 'CN': '中国',
        'HONG KONG': '中国香港', 'HK': '中国香港',
        'TAIWAN': '中国台湾', 'TW': '中国台湾',
        'MACAO': '中国澳门', 'MACAU': '中国澳门'
    };

    var DEFAULT_SHIPPING = {
        'SEA': '海运', 'BY SEA': '海运', 'SEA FREIGHT': '海运', 'OCEAN': '海运', 'OCEAN FREIGHT': '海运',
        'LCL': '海运拼箱', 'FCL': '海运整柜',
        'AIR': '空运', 'BY AIR': '空运', 'AIR FREIGHT': '空运',
        'TRUCK': '卡车', 'BY TRUCK': '卡车', 'FAST TRUCK': '快运卡车', 'EXPRESS TRUCK': '快运卡车',
        'RAIL': '铁路', 'BY RAIL': '铁路', 'RAILWAY': '铁路',
        'EXPRESS': '快递', 'COURIER': '快递',
        'DHL': 'DHL 快递', 'UPS': 'UPS 快递', 'FEDEX': 'FedEx 快递', 'TNT': 'TNT 快递',
        'EMS': 'EMS', 'POST': '邮政'
    };

    function clone(src) {
        var dst = {};
        for (var key in src) {
            if (Object.prototype.hasOwnProperty.call(src, key)) dst[key] = src[key];
        }
        return dst;
    }

    var COUNTRY = clone(DEFAULT_COUNTRY);
    var SHIPPING = clone(DEFAULT_SHIPPING);

    function normalizeKey(value) {
        return String(value === null || value === undefined ? '' : value)
            .trim().toUpperCase().replace(/\s+/g, ' ');
    }

    // 一行一条：英文=中文。分隔符支持 =、＝、制表符；# 开头为注释。
    function parseLines(text, target) {
        String(text || '').split(/\r?\n/).forEach(function (line) {
            var row = line.trim();
            if (!row || row.charAt(0) === '#') return;

            var idx = row.search(/[=＝\t]/);
            if (idx <= 0) return;

            var key = normalizeKey(row.slice(0, idx));
            var value = row.slice(idx + 1).trim();
            if (!key || !value) return;

            target[key] = value;
        });
        return target;
    }

    function toText(dict) {
        return Object.keys(dict).sort().map(function (key) {
            return key + '=' + dict[key];
        }).join('\n');
    }

    function pick(dict, value) {
        var v = String(value === null || value === undefined ? '' : value).trim();
        if (!v) return '';
        if (/[\u4e00-\u9fa5]/.test(v)) return v;

        var key = normalizeKey(v);
        if (dict[key]) return dict[key];

        // 复合写法兜底（如 "FAST TRUCK (LTL)"）：长词优先，2 字母代码不参与，避免 US 命中 RUSSIA。
        var keys = Object.keys(dict).sort(function (a, b) { return b.length - a.length; });
        for (var i = 0; i < keys.length; i++) {
            if (keys[i].length < 3) continue;
            if (key.indexOf(keys[i]) >= 0) return dict[keys[i]];
        }
        return v;
    }

    // 拉取系统设置里维护的字典，覆盖内置值；接口不可用时静默回落到内置。
    var ready = Promise.resolve();
    if (typeof fetch === 'function') {
        ready = fetch('/api/i18n', { headers: { 'Accept': 'application/json' } })
            .then(function (resp) { return resp.ok ? resp.json() : null; })
            .then(function (res) {
                var data = (res && res.data) || {};
                parseLines(data.country, COUNTRY);
                parseLines(data.shipping, SHIPPING);
            })
            .catch(function () { /* 离线或接口异常时用内置字典 */ });
    }

    window.OrderI18N = {
        ready: ready,
        country: function (value) { return pick(COUNTRY, value); },
        shipping: function (value) { return pick(SHIPPING, value); },
        // 设置页用来预填 / 恢复内置字典。
        defaults: function () {
            return { country: toText(DEFAULT_COUNTRY), shipping: toText(DEFAULT_SHIPPING) };
        }
    };
})();
