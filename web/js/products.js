
$(function () {
    const api = '/api/skus';
    const modal = new bootstrap.Modal(document.getElementById('skuModal'));
    let skus = [];

    // loadProductCategories 把报关品类填进下拉，供 SKU 挂品类（报关资料按品类出行）。
    async function loadProductCategories() {
        try {
            const result = await $.getJSON('/api/product-categories', { status: 'active' });
            categories((result && result.data) || []);
        } catch (err) {
            categories([]);
        }
    }

    function categories(list) {
        const sel = $('#productCategoryId');
        sel.empty().append('<option value="0">（未指定）</option>');
        list.forEach(c => {
            sel.append(`<option value="${c.id}">${esc(c.name_zh)}${c.code ? '（' + esc(c.code) + '）' : ''}</option>`);
        });
    }

    function esc(value) {
        return String(value ?? '').replace(/[&<>"']/g, c => ({
            '&': '&amp;', '<': '&lt;', '>': '&gt;',
            '"': '&quot;', "'": '&#39;'
        }[c]));
    }

    function num(id) {
        return Number($('#' + id).val()) || 0;
    }

    function money(value, currency) {
        return `${esc(currency)} ${Number(value || 0).toFixed(2)}`;
    }

    // nextSortOrder 新增 SKU 时的默认排序值：排在当前最后一条之后。
    function nextSortOrder() {
        if (!skus.length) return 10;
        return Math.max(...skus.map(s => Number(s.sort_order) || 0)) + 10;
    }

    function render() {
        const tbody = $('#skuTableBody');
        tbody.empty();

        if (!skus.length) {
            tbody.html('<tr><td colspan="15" class="text-center text-secondary py-4">暂无 SKU 数据</td></tr>');
            $('#skuCount').text('共 0 个 SKU');
            return;
        }

        skus.forEach(s => {
            const image = s.image
                ? `<img src="${esc(s.image)}" alt="" style="width:48px;height:48px;object-fit:contain">`
                : '<span class="text-secondary">无图片</span>';

            const status = s.status === 'active'
                ? '<span class="badge text-bg-success">启用</span>'
                : '<span class="badge text-bg-secondary">停用</span>';

            const box = `${s.box_length} × ${s.box_width} × ${s.box_height} ${esc(s.box_unit)}`;

            tbody.append(`
                <tr>
                    <td>${image}</td>
                    <td class="text-secondary">${Number(s.sort_order) || 0}</td>
                    <td class="fw-semibold">${esc(s.sku_code)}</td>
                    <td>${esc(s.product_name_zh)}</td>
                    <td>${esc(s.product_name)}</td>
                    <td>${esc(s.short_name)}</td>
                    <td>${esc(s.model)}</td>
                    <td>${esc(s.color)}</td>
                    <td>${money(s.price, s.currency)}</td>
                    <td>${s.price_usd > 0 ? '$ ' + Number(s.price_usd).toFixed(2) : ''}</td>
                    <td>${s.declare_price > 0 ? '$ ' + Number(s.declare_price).toFixed(2) : ''}</td>
                    <td>${esc(s.pack_quantity)}</td>
                    <td>${box}</td>
                    <td>${status}</td>
                    <td class="text-nowrap">
                        <button class="btn btn-sm btn-outline-primary edit-sku" data-id="${s.id}">编辑</button>
                        <button class="btn btn-sm btn-outline-danger delete-sku" data-id="${s.id}">删除</button>
                    </td>
                </tr>
            `);
        });

        $('#skuCount').text(`共 ${skus.length} 个 SKU`);
    }

    async function loadSKUs() {
        $('#skuTableBody').html('<tr><td colspan="15" class="text-center py-4">正在加载...</td></tr>');

        try {
            const result = await $.getJSON(api, {
                keyword: $('#keyword').val().trim(),
                status: $('#statusFilter').val()
            });

            if (!result.success) throw new Error(result.message || '读取 SKU 失败');
            skus = result.data || [];
            render();
        } catch (err) {
            $('#skuTableBody').html(
                `<tr><td colspan="15" class="text-danger text-center py-4">${esc(err.responseJSON?.message || err.message || '读取失败')}</td></tr>`
            );
        }
    }

    function resetForm() {
        $('#skuForm')[0].reset();
        $('#skuId').val('');
        $('#skuImage').val('');
        $('#skuImagePreview').hide().attr('src', '');
        $('#sortOrder').val(nextSortOrder());
        $('#packQuantity').val('1');
        $('#price').val('0');
        $('#priceUsd').val('0');
        $('#declarePrice').val('0');
        $('#boxLength,#boxWidth,#boxHeight,#boxWeight').val('0');
        $('#currency').val('CNY');
        $('#boxUnit').val('cm');
        $('#weightUnit').val('kg');
        $('#skuStatus').val('active');
        $('#category').val('finished');
        $('#unit').val('pcs');
        $('#productCategoryId').val('0');
        $('#asin').val('');
        $('#fnsku').val('');
    }

    function fillForm(s) {
        $('#skuId').val(s.id);
        $('#sortOrder').val(s.sort_order || 0);
        $('#skuCode').val(s.sku_code);
        $('#productName').val(s.product_name);
        $('#productNameZh').val(s.product_name_zh);
        $('#shortName').val(s.short_name || '');
        $('#series').val(s.series || '');
        $('#sizeSpec').val(s.size_spec || '');
        $('#piColor').val(s.pi_color || '');
        $('#category').val(s.category || 'finished');
        $('#unit').val(s.unit || 'pcs');
        $('#hsCode').val(s.hs_code || '');
        $('#productCategoryId').val(String(s.product_category_id || 0));
        $('#asin').val(s.asin || '');
        $('#fnsku').val(s.fnsku || '');
        $('#model').val(s.model);
        $('#price').val(s.price);
        $('#priceUsd').val(s.price_usd || 0);
        $('#declarePrice').val(s.declare_price || 0);
        $('#currency').val(s.currency || 'CNY');
        $('#color').val(s.color);
        $('#packQuantity').val(s.pack_quantity);
        $('#boxLength').val(s.box_length);
        $('#boxWidth').val(s.box_width);
        $('#boxHeight').val(s.box_height);
        $('#boxUnit').val(s.box_unit || 'cm');
        $('#boxWeight').val(s.box_weight);
        $('#weightUnit').val(s.weight_unit || 'kg');
        $('#skuStatus').val(s.status);
        $('#skuImage').val(s.image || '');
        $('#description').val(s.description || '');
        $('#remark').val(s.remark || '');

        if (s.image) {
            $('#skuImagePreview').attr('src', s.image).show();
        } else {
            $('#skuImagePreview').hide();
        }
    }

    $('#addSkuBtn').on('click', function () {
        resetForm();
        $('#skuModalTitle').text('新增 SKU');
        modal.show();
    });

    $('#searchBtn,#refreshBtn').on('click', loadSKUs);
    $('#statusFilter').on('change', loadSKUs);

    $('#keyword').on('keydown', function (e) {
        if (e.key === 'Enter') loadSKUs();
    });

    $('#skuTableBody').on('click', '.edit-sku', async function () {
        const id = Number($(this).data('id'));

        try {
            const result = await $.getJSON(`${api}/${id}`);
            if (!result.success) throw new Error(result.message || '读取失败');

            resetForm();
            fillForm(result.data);
            $('#skuModalTitle').text('编辑 SKU');
            modal.show();
        } catch (err) {
            alert(err.responseJSON?.message || err.message || '读取 SKU 失败');
        }
    });

    $('#skuTableBody').on('click', '.delete-sku', async function () {
        const id = Number($(this).data('id'));
        const sku = skus.find(s => Number(s.id) === id);
        if (!sku) return;

        if (!confirm(`确定删除 SKU「${sku.sku_code} / ${sku.product_name_zh}」吗？`)) return;

        try {
            const result = await $.ajax({
                url: `${api}/${id}`,
                method: 'DELETE',
                dataType: 'json'
            });

            if (!result.success) throw new Error(result.message || '删除失败');
            await loadSKUs();
        } catch (err) {
            alert(err.responseJSON?.message || err.message || '删除失败');
        }
    });

    $('#skuImageFile').on('change', async function () {
        const file = this.files?.[0];
        if (!file) return;

        const formData = new FormData();
        formData.append('image', file);

        try {
            const result = await $.ajax({
                url: `${api}/upload-image`,
                method: 'POST',
                data: formData,
                processData: false,
                contentType: false,
                dataType: 'json'
            });

            if (!result.success || !result.data?.image) {
                throw new Error(result.message || '图片上传失败');
            }

            $('#skuImage').val(result.data.image);
            $('#skuImagePreview').attr('src', result.data.image).show();
        } catch (err) {
            this.value = '';
            alert(err.responseJSON?.message || err.message || '图片上传失败');
        }
    });

    $('#skuForm').on('submit', async function (e) {
        e.preventDefault();

        const id = $('#skuId').val();
        const payload = {
            sort_order: Number($('#sortOrder').val()) || 0,
            sku_code: $('#skuCode').val().trim(),
            product_name: $('#productName').val().trim(),
            product_name_zh: $('#productNameZh').val().trim(),
            short_name: $('#shortName').val().trim(),
            series: $('#series').val().trim(),
            size_spec: $('#sizeSpec').val().trim(),
            pi_color: $('#piColor').val().trim(),
            category: $('#category').val(),
            unit: $('#unit').val().trim(),
            hs_code: $('#hsCode').val().trim(),
            product_category_id: Number($('#productCategoryId').val()) || 0,
            asin: $('#asin').val().trim(),
            fnsku: $('#fnsku').val().trim(),
            model: $('#model').val().trim(),
            price: num('price'),
            price_usd: num('priceUsd'),
            declare_price: num('declarePrice'),
            currency: $('#currency').val(),
            color: $('#color').val().trim(),
            pack_quantity: num('packQuantity'),
            box_length: num('boxLength'),
            box_width: num('boxWidth'),
            box_height: num('boxHeight'),
            box_unit: $('#boxUnit').val(),
            box_weight: num('boxWeight'),
            weight_unit: $('#weightUnit').val(),
            image: $('#skuImage').val(),
            status: $('#skuStatus').val(),
            description: $('#description').val().trim(),
            remark: $('#remark').val().trim()
        };

        if (!payload.sku_code || !payload.product_name ||
            !payload.product_name_zh || !payload.model) {
            alert('请填写 SKU 编码、产品中英文名和型号');
            return;
        }

        $('#saveSkuBtn').prop('disabled', true).text('正在保存...');

        try {
            const result = await $.ajax({
                url: id ? `${api}/${id}` : api,
                method: id ? 'PUT' : 'POST',
                contentType: 'application/json',
                data: JSON.stringify(payload),
                dataType: 'json'
            });

            if (!result.success) throw new Error(result.message || '保存失败');

            modal.hide();
            await loadSKUs();
        } catch (err) {
            alert(err.responseJSON?.message || err.message || '保存 SKU 失败');
        } finally {
            $('#saveSkuBtn').prop('disabled', false).text('保存 SKU');
        }
    });

    loadProductCategories();
    loadSKUs();
});
 