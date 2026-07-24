/**
 * Sale → Stock Move Hook
 * 
 * Runs asynchronously after every Sale insert.
 * For each Sale Item, creates a Stock Move on the Product
 * with negative qty_change to decrement inventory.
 *
 * Global API available:
 *   __kora_event__.doc     — the sale document (with items array)
 *   kora.getDoc(dt, name)  — fetch a document
 *   kora.saveDoc(dt, doc)  — save a document
 *   kora.log.info/warn/error(msg) — logging
 *
 * Setup:
 *   1. KORA_SCRIPTS_ENABLED=true
 *   2. Register via Admin UI at /workspace/admin/scripts:
 *      - Name: sale_stock_update
 *      - Script Type: doc_event
 *      - DocType: Sale
 *      - Event: after_insert
 *      - Is Active: yes
 */
var doc = __kora_event__.doc;
var items = doc.items;

if (!items || !Array.isArray(items) || items.length === 0) {
  return;
}

for (var i = 0; i < items.length; i++) {
  var item = items[i];
  var productName = item.product;
  if (!productName) continue;

  try {
    var product = kora.getDoc("Product", productName);
    if (!product) {
      kora.log.warn("Product not found: " + productName);
      continue;
    }

    var stockMove = {
      movement_type: "Sale Issue",
      qty_change: -(parseFloat(item.quantity) || 1),
      unit_cost: parseFloat(item.unit_price) || 0,
      warehouse: product.default_warehouse || "",
      reference: doc.name,
      movement_date: new Date().toISOString().split("T")[0],
      notes: "Sale: " + (doc.receipt_number || doc.name)
    };

    var existingMoves = product.stock_moves || [];
    existingMoves.push(stockMove);
    product.stock_moves = existingMoves;

    kora.saveDoc("Product", product);
    kora.log.info(
      "Stock updated: " + productName +
      " qty=" + stockMove.qty_change +
      " (sale " + doc.name + ")"
    );
  } catch (e) {
    kora.log.error("Stock update failed for " + productName + ": " + e);
  }
}
