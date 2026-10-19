$(document).ready(function () {

    /*
     * 当前页面高亮
     */

    const path = window.location.pathname;

    $(".nav-item").removeClass("active");

    // 页面路径前缀 → 导航项 data-page（更具体的路径排在前面）。
    const navRoutes = [
        ["/purchase-orders", "purchase-orders"],
        ["/production-orders", "production-orders"],
        ["/product-match-rules", "product-match-rules"],
        ["/product-categories", "product-categories"],
        ["/product-bom", "product-bom"],
        ["/products", "products"],
        ["/orderupload", "ai"],
        ["/orders", "orders"],
        ["/shipments", "shipments"],
        ["/material-issues", "material-issues"],
        ["/materials", "materials"],
        ["/inventory", "inventory"],
        ["/proforma-invoices", "proforma-invoices"],
        ["/payments", "payments"],
        ["/declaration-templates", "declaration-templates"],
        ["/finance", "finance"],
        ["/statistics", "statistics"],
        ["/settings", "settings"],
        ["/operation-logs", "operation-logs"],
        ["/users", "users"]
    ];

    const matched = navRoutes.find(function (route) {
        return path.startsWith(route[0]);
    });

    if (matched) {

        $('[data-page="' + matched[1] + '"]')
            .addClass("active");

    } else if (
        path === "/" ||
        path === ""
    ) {

        $('[data-page="dashboard"]')
            .addClass("active");

    }


    /*
     * 移动端侧边栏
     */

    $("#sidebarToggle").on("click", function () {

        $(".sidebar")
            .toggleClass("open");

    });


    /*
     * 点击内容区域关闭移动端菜单
     */

    $(".content").on("click", function () {

        $(".sidebar")
            .removeClass("open");

    });

});