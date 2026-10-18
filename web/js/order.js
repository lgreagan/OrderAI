$(function () {

    let selectedFile = null;

    $("#orderImage").on("change", function () {

        const file = this.files[0];

        if (!file) {
            selectedFile = null;
            $("#uploadButton").prop("disabled", true);
            $("#previewContainer").hide();
            return;
        }

        const allowedTypes = [
            "image/jpeg",
            "image/png",
            "image/webp"
        ];

        if (!allowedTypes.includes(file.type)) {

            alert("请选择 JPG、PNG 或 WEBP 图片");

            this.value = "";

            selectedFile = null;

            $("#uploadButton").prop("disabled", true);

            return;
        }

        selectedFile = file;

        // 显示预览
        const reader = new FileReader();

        reader.onload = function (e) {

            $("#previewImage")
                .attr("src", e.target.result);

            $("#previewContainer").show();
        };

        reader.readAsDataURL(file);

        $("#uploadButton")
            .prop("disabled", false);

        $("#uploadStatus").html("");
    });


    $("#uploadButton").on("click", function () {

        if (!selectedFile) {
            return;
        }

        const formData = new FormData();

        formData.append(
            "order_image",
            selectedFile
        );

        const button = $(this);

        button
            .prop("disabled", true)
            .text("正在上传...");

        $("#uploadStatus").html(
            '<div class="alert alert-info">正在上传订单图片...</div>'
        );

        $.ajax({

            url: "/api/order/upload",

            type: "POST",

            data: formData,

            processData: false,

            contentType: false,

            success: function (res) {

                if (res.success) { 

    $("#uploadStatus").html(
        '<div class="alert alert-success">' +
        '图片上传成功' +
        '</div>' +

        '<button ' +
        'id="recognizeButton" ' +
        'class="btn btn-success w-100 mt-2">' +
        'AI识别订单' +
        '</button>'
    );

    $("#recognizeButton").on("click", function () {

        recognizeOrder(res.filename);

    });
                } else {

                    $("#uploadStatus").html(
                        '<div class="alert alert-danger">' +
                        res.message +
                        '</div>'
                    );

                }

            },

            error: function (xhr) {

                let message = "上传失败";

                if (
                    xhr.responseJSON &&
                    xhr.responseJSON.message
                ) {
                    message =
                        xhr.responseJSON.message;
                }

                $("#uploadStatus").html(
                    '<div class="alert alert-danger">' +
                    message +
                    '</div>'
                );

            },

            complete: function () {

                button
                    .prop("disabled", false)
                    .text("上传订单图片");

            }

        });

    });

});

function recognizeOrder(filename) {

    // 记住上传给 AI 识别的这张图，确认订单时写入 ai_results.image_path，
    // PI 导出会把它贴到对应分组的 A:B 区域。
    currentImageFilename = filename;

    const button = $("#recognizeButton");

    button
        .prop("disabled", true)
        .text("AI正在识别订单，请稍候...");

    $("#uploadStatus").html(
        '<div class="alert alert-info">' +
        '正在将订单图片发送给 DeepSeek...<br>' +
        '正在识别订单号、产品、规格和数量...' +
        '</div>'
    );

    $.ajax({

        url: "/api/order/recognize",

        type: "POST",

        data: {
            filename: filename
        },

        success: function (res) {

            if (!res.success) {

                $("#uploadStatus").html(
                    '<div class="alert alert-danger">' +
                    res.message +
                    '</div>'
                );

                button
                    .prop("disabled", false)
                    .text("AI识别订单");

                return;
            }

            showAIResult(res.data);

        },

        error: function (xhr) {

            let message = "AI识别失败";

            if (
                xhr.responseJSON &&
                xhr.responseJSON.message
            ) {
                message =
                    xhr.responseJSON.message;
            }

            $("#uploadStatus").html(
                '<div class="alert alert-danger">' +
                message +
                '</div>'
            );

            button
                .prop("disabled", false)
                .text("AI识别订单");
        }

    });
}

let currentAIOrder = null;
let currentImageFilename = "";
let currentAIResultJSON = "";

function showAIResult(order) {

    let html = "";
    currentAIOrder = order;
    html += `
        <div class="card mt-4">
            <div class="card-body">

                <h4 class="mb-4">
                    AI 订单识别结果
                </h4>

                <div class="row mb-4">

                    <div class="col-md-3">
                        <strong>订单号</strong>
                        <div>${escapeHtml(order.order_number)}</div>
                    </div>

                    <div class="col-md-3">
                        <strong>国家</strong>
                        <div>${escapeHtml(order.country)}</div>
                    </div>

                    <div class="col-md-3">
                        <strong>运输方式</strong>
                        <div>${escapeHtml(order.shipping_method)}</div>
                    </div>

                    <div class="col-md-3">
                        <strong>客户</strong>
                        <div>${escapeHtml(order.customer)}</div>
                    </div>

                </div>

                <div class="table-responsive">

                    <table class="table table-bordered align-middle">

                        <thead class="table-light">

                            <tr>
                                <th>行</th>
                                <th>产品</th>
                                <th>规格</th>
                                <th>数量</th>
                                <th>单位</th>
                                <th>包装</th>
                                <th>实际数量</th>
                                <th>客户原始数量</th>
                            </tr>

                        </thead>

                        <tbody>
    `;

    if (order.items && order.items.length > 0) {

        order.items.forEach(function (item) {

            html += `
                <tr>

                    <td>
                        ${item.line_number}
                    </td>

                    <td>
                        ${escapeHtml(item.product_name)}
                        <div class="small text-muted">
                            ${escapeHtml(item.description)}
                        </div>
                    </td>

                    <td>
                        ${escapeHtml(item.specification)}
                    </td>

                    <td>
                        ${item.quantity}
                    </td>

                    <td>
                        ${escapeHtml(item.unit)}
                    </td>

                    <td>
                        ${item.pack_size}
                    </td>

                    <td>
                        <strong>
                            ${item.actual_quantity}
                        </strong>
                    </td>

                    <td>
                        ${escapeHtml(item.declared_quantity)}
                    </td>

                </tr>
            `;

        });

    } else {

        html += `
            <tr>
                <td colspan="8" class="text-center">
                    未识别到产品
                </td>
            </tr>
        `;
    }

    html += `
                        </tbody>

                    </table>

                </div>
    `;

    if (
        order.notes &&
        order.notes.length > 0
    ) {

        html += `
            <div class="alert alert-warning mt-3">

                <strong>
                    AI 注意事项
                </strong>

                <ul class="mb-0 mt-2">
        `;

        order.notes.forEach(function (note) {

            html += `
                <li>
                    ${escapeHtml(note)}
                </li>
            `;

        });

        html += `
                </ul>

            </div>
        `;
    }

    html += `
                <div class="mt-4" id="matchPreview"></div>

                <div class="mt-4">

                    <button
                        class="btn btn-primary"
                        onclick="confirmOrder()"
                    >
                        确认识别结果
                    </button>

                    <button
                        class="btn btn-outline-secondary ms-2"
                        onclick="location.reload()"
                    >
                        重新识别
                    </button>

                </div>

            </div>
        </div>
    `;

    $("#uploadStatus").after(html);

    // 识别结果展示后立即计算 SKU 匹配结果，供用户确认前查看
    loadMatchPreview(order.items);
}

// ========================================
// 订单行 SKU 匹配预览
// ========================================

function loadMatchPreview(items) {

    const box = $("#matchPreview");

    if (!items || items.length === 0) {
        box.html(
            '<div class="alert alert-warning mb-0">没有可匹配的订单行</div>'
        );
        return;
    }

    box.html('<div class="text-muted">正在匹配单品 SKU...</div>');

    $.ajax({

        url: "/api/order/match",

        type: "POST",

        contentType: "application/json",

        data: JSON.stringify({ items: items }),

        success: function (res) {

            if (!res.success) {

                box.html(
                    '<div class="alert alert-danger mb-0">' +
                    escapeHtml(res.message || "SKU 匹配失败") +
                    '</div>'
                );

                return;
            }

            renderMatchPreview(res.data || [], res.summary || {});
        },

        error: function (xhr) {

            let message = "SKU 匹配失败";

            if (xhr.responseJSON && xhr.responseJSON.message) {
                message += "：" + xhr.responseJSON.message;
            }

            box.html(
                '<div class="alert alert-danger mb-0">' +
                escapeHtml(message) +
                '</div>'
            );
        }

    });
}

function renderMatchPreview(previews, summary) {

    const total = summary.total || previews.length;
    const direct = summary.direct_count || 0;
    const rule = summary.rule_count || 0;
    const unmatched = summary.unmatched_count || 0;

    let rows = "";

    previews.forEach(function (preview) {

        const skus = preview.skus || [];
        const rowspan = skus.length > 0 ? skus.length : 1;

        const itemCells = `
            <td rowspan="${rowspan}">${preview.line_number}</td>
            <td rowspan="${rowspan}">
                ${escapeHtml(preview.product_name)}
                ${preview.specification
                    ? '<div class="small text-muted">' +
                      escapeHtml(preview.specification) + '</div>'
                    : ''}
            </td>
            <td rowspan="${rowspan}">
                ${formatNumber(preview.quantity)}
                ${escapeHtml(preview.unit)}
                ${preview.carton_converted
                    ? '<div class="small text-muted">= ' +
                      formatNumber(preview.base_quantity) + ' 套</div>'
                    : ''}
            </td>
            <td rowspan="${rowspan}">${matchLabel(preview)}</td>
        `;

        if (skus.length === 0) {

            rows += `
                <tr>
                    ${itemCells}
                    <td colspan="3" class="text-danger">
                        ${escapeHtml(preview.message || "未匹配到 SKU")}
                    </td>
                </tr>
            `;

            return;
        }

        skus.forEach(function (sku, index) {

            rows += `
                <tr>
                    ${index === 0 ? itemCells : ""}
                    <td>
                        ${escapeHtml(sku.sku_code)}
                        <div class="small text-muted">
                            ${escapeHtml(sku.product_name_zh || sku.product_name)}
                        </div>
                        <div class="small text-muted">
                            ${escapeHtml(sku.model || "-")}
                            ${sku.color ? " · " + escapeHtml(sku.color) : ""}
                        </div>
                    </td>
                    <td>${formatNumber(sku.quantity_per_set)}</td>
                    <td>
                        <strong>${formatNumber(sku.required_quantity)}</strong>
                    </td>
                </tr>
            `;
        });
    });

    const html = `
        <h4 class="mb-3">SKU 匹配结果</h4>

        <div class="alert ${unmatched > 0 ? "alert-warning" : "alert-success"} py-2">
            共 ${total} 行：单品直配 <strong>${direct}</strong> 行，
            组合规则 <strong>${rule}</strong> 行，
            未匹配 <strong>${unmatched}</strong> 行。
            ${unmatched > 0
                ? "<br>未匹配的订单行不会写入匹配结果，订单仍可正常入库。"
                : ""}
        </div>

        <div class="table-responsive">
            <table class="table table-bordered align-middle">
                <thead class="table-light">
                    <tr>
                        <th>行</th>
                        <th>客户商品 / 规格</th>
                        <th>订单数量</th>
                        <th>匹配方式</th>
                        <th>匹配 SKU</th>
                        <th>每套数量</th>
                        <th>需求数量</th>
                    </tr>
                </thead>
                <tbody>${rows}</tbody>
            </table>
        </div>
    `;

    $("#matchPreview").html(html);
}

function matchLabel(preview) {

    if (!preview.matched) {
        return '<span class="badge text-bg-danger">未匹配</span>';
    }

    if (preview.match_type === "sku") {
        return '<span class="badge text-bg-success">单品直配</span>';
    }

    return (
        '<span class="badge text-bg-primary">组合规则</span>' +
        '<div class="small text-muted">' +
        escapeHtml(preview.rule_code || "-") +
        '</div>'
    );
}

function formatNumber(value) {

    const number = Number(value || 0);

    if (Number.isInteger(number)) {
        return String(number);
    }

    return number.toFixed(3).replace(/0+$/, "").replace(/\.$/, "");
}


function escapeHtml(value) {

    if (value === null || value === undefined) {
        return "";
    }

    return String(value)
        .replace(/&/g, "&amp;")
        .replace(/</g, "&lt;")
        .replace(/>/g, "&gt;")
        .replace(/"/g, "&quot;")
        .replace(/'/g, "&#039;");
}


function confirmOrder() {

    if (!currentAIOrder) {
        alert("当前没有可确认的订单");
        return;
    }

    if (!currentAIOrder.order_number) {
        alert("订单号不能为空");
        return;
    }

    if (!currentAIOrder.items || currentAIOrder.items.length === 0) {
        alert("订单中没有产品明细");
        return;
    }

    // 防止重复点击
    const btn = $("#confirmOrderBtn");

    if (btn.length > 0) {
        btn.prop("disabled", true);
        btn.text("正在入库...");
    }

    const requestData = {
        order: currentAIOrder,

        // 保存AI原始识别结果
        raw_json: currentAIResultJSON || JSON.stringify(
            currentAIOrder,
            null,
            2
        ),

        // 原始订单图片
        image_path: currentImageFilename
            ? "/uploads/" + currentImageFilename
            : "",

        provider: "deepseek",

        model: "deepseek-flash"
    };

    $.ajax({
        url: "/api/orders/confirm",
        type: "POST",
        contentType: "application/json",
        data: JSON.stringify(requestData),

        success: function (res) {

            if (!res.success) {

                alert(
                    "订单入库失败：" +
                    (res.message || "未知错误")
                );

                return;
            }

            let message = "订单确认并入库成功！";

            if (res.data && res.data.unmatched_line_count > 0) {
                message +=
                    "\n其中 " +
                    res.data.unmatched_line_count +
                    " 行未匹配到 SKU，已按原样保存订单。";
            }

            alert(message);

            // 跳转到订单管理页面
            window.location.href = "/orders";
        },

        error: function (xhr) {

            let message = "订单入库失败";

            if (xhr.responseJSON && xhr.responseJSON.message) {
                message += "：" + xhr.responseJSON.message;
            }

            alert(message);
        },

        complete: function () {

            if (btn.length > 0) {
                btn.prop("disabled", false);
                btn.text("确认并入库");
            }
        }
    });
}