/*
 * 采购下单文案：把采购清单拼成一句可以直接发微信 / 短信 / 邮件的话。
 *
 * 形如：
 *   西瓜球：6CM薄荷绿120个，草绿120个，7CM薄荷绿64个，草绿64个，一共368个；
 *   专利球：6CM薄荷绿1440个，草绿144个，橙色576个，8CM薄荷绿704个，草绿192个，橙色256个，一共3312个；
 *   加起来是3680个。
 *
 * 输入： [{ series, name, color, qty }]
 *   series —— 系列代码（V1/V2），用于排序，可空
 *   name   —— 产品中文名（「西瓜球7厘米草绿」），尺寸与系列名都从这里解析
 *   color  —— 颜色，名称里没带颜色时兜底
 */
(function () {
    // 颜色固定顺序，跟口头报数的习惯一致；没列到的按名称排在后面。
    var COLOR_ORDER = ['薄荷绿', '草绿', '橙色', '蓝色', '绿色', '红色', '黑色', '白色', '黄色', '粉色', '紫色', '灰色'];

    function fmtQty(value) {
        var n = Number(value || 0);
        return n.toFixed(3).replace(/\.?0+$/, '');
    }

    function colorRank(color) {
        var i = COLOR_ORDER.indexOf(color);
        return i < 0 ? 100 : i;
    }

    function sizeRank(size) {
        if (!size) return 1e9;
        var m = String(size).match(/(\d+(?:\.\d+)?)/);
        return m ? Number(m[1]) : 1e8;
    }

    // 系列排序：V1 < V2 < V10，空值最后。
    function seriesRank(key) {
        var m = String(key).match(/^(.*?)(\d+)\s*$/);
        return m ? [m[1], parseInt(m[2], 10)] : [String(key), -1];
    }
    function compareSeries(a, b) {
        if (!a && !b) return 0;
        if (!a) return 1;
        if (!b) return -1;
        var ra = seriesRank(a), rb = seriesRank(b);
        if (ra[0] !== rb[0]) return ra[0] < rb[0] ? -1 : 1;
        return ra[1] - rb[1];
    }

    // 「西瓜球7厘米草绿」拆成 系列=西瓜球、尺寸=7CM、颜色=草绿。
    function parse(item) {
        var name = String(item.name || '').trim();
        var color = String(item.color || '').trim();
        var matched = name.match(/(\d+(?:\.\d+)?)\s*(?:厘米|cm|CM|Cm)/);

        var seriesName = name;
        var size = '';
        var colorText = '';

        if (matched) {
            size = matched[1] + 'CM';
            seriesName = name.slice(0, matched.index).trim() || name;
            colorText = name.slice(matched.index + matched[0].length).trim();
        } else {
            // 没有尺寸的产品（刷子、牙刷等）单独归到「其它」，不硬套系列名。
            seriesName = '';
        }
        if (!colorText) colorText = color;

        // 无尺寸的产品直接报全名，避免「刷子橙色」这种别扭说法。
        var label = size ? (size + colorText) : name;

        return {
            key: size ? (item.series || seriesName) : '',
            seriesName: seriesName,
            size: size,
            color: colorText,
            label: label,
            qty: Number(item.qty || 0)
        };
    }

    function build(items) {
        var list = (items || []).map(parse).filter(function (it) { return it.qty > 0; });
        if (!list.length) return '';

        var seriesMap = {}, seriesKeys = [];
        list.forEach(function (it) {
            var s = seriesMap[it.key];
            if (!s) {
                s = seriesMap[it.key] = { name: it.seriesName, items: {}, order: [] };
                seriesKeys.push(it.key);
            }
            var g = s.items[it.label];
            if (!g) {
                g = s.items[it.label] = { label: it.label, size: it.size, color: it.color, qty: 0 };
                s.order.push(it.label);
            }
            g.qty += it.qty;
        });

        seriesKeys.sort(compareSeries);

        var parts = [], total = 0;
        seriesKeys.forEach(function (key) {
            var s = seriesMap[key];
            var lines = s.order.map(function (label) { return s.items[label]; });

            // 同系列内：尺寸由小到大，同尺寸按固定色序。
            lines.sort(function (a, b) {
                var d = sizeRank(a.size) - sizeRank(b.size);
                if (d !== 0) return d;
                var c = colorRank(a.color) - colorRank(b.color);
                if (c !== 0) return c;
                return a.label < b.label ? -1 : (a.label > b.label ? 1 : 0);
            });

            var sum = 0;
            var prevSize = null;
            var texts = lines.map(function (g) {
                // 同一尺寸只在第一项前写一次（7CM薄荷绿120个，草绿120个），句子更短。
                var label = g.label;
                if (g.size && g.size === prevSize) label = g.label.slice(g.size.length);
                prevSize = g.size;

                sum += g.qty;
                return label + fmtQty(g.qty) + '个';
            });
            total += sum;

            var head = s.name ? (s.name + '：') : '';
            parts.push(head + texts.join('，') + '，一共' + fmtQty(sum) + '个');
        });

        if (parts.length === 1) return parts[0] + '。';
        return parts.join('；') + '；加起来是' + fmtQty(total) + '个。';
    }

    function fallbackCopy(text) {
        try {
            var area = document.createElement('textarea');
            area.value = text;
            area.setAttribute('readonly', '');
            area.style.position = 'fixed';
            area.style.left = '-9999px';
            document.body.appendChild(area);
            area.select();
            var ok = document.execCommand && document.execCommand('copy');
            document.body.removeChild(area);
            return !!ok;
        } catch (e) {
            return false;
        }
    }

    // 复制到剪贴板：优先 Clipboard API，非 https / 老浏览器回落到 execCommand。
    function copy(text) {
        if (!text) return Promise.resolve(false);
        if (navigator.clipboard && navigator.clipboard.writeText) {
            return navigator.clipboard.writeText(text).then(function () {
                return true;
            }, function () {
                return fallbackCopy(text);
            });
        }
        return Promise.resolve(fallbackCopy(text));
    }

    window.OrderText = { build: build, copy: copy };
})();
