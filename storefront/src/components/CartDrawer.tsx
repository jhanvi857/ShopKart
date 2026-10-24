import React from 'react';
import { Cart } from '../types';
import { X, Trash2, ArrowRight, ShoppingCart, ShieldCheck } from 'lucide-react';

interface CartDrawerProps {
  isOpen: boolean;
  onClose: () => void;
  cart: Cart | null;
  onRemoveItem: (productId: string) => void;
  onClearCart: () => void;
  onOpenCheckout: () => void;
}

export const CartDrawer: React.FC<CartDrawerProps> = ({
  isOpen,
  onClose,
  cart,
  onRemoveItem,
  onClearCart,
  onOpenCheckout,
}) => {
  if (!isOpen) return null;

  const items = cart?.items || [];
  const total = cart?.total || 0;

  // Approximate standard Indian retail savings (mrp vs price)
  const estimatedSavings = Math.round(total * 0.35);

  return (
    <div className="fixed inset-0 z-50 flex justify-end bg-black/60 select-none">
      <div className="w-full max-w-md bg-white h-full shadow-2xl flex flex-col justify-between">
        {/* Cart Header */}
        <div className="p-4 bg-[#2874f0] text-white flex items-center justify-between">
          <div className="flex items-center gap-2">
            <ShoppingCart className="w-4 h-4" />
            <h3 className="font-bold text-base">My Cart ({items.length} items)</h3>
          </div>
          <button
            onClick={onClose}
            className="p-1 hover:bg-white/20 rounded text-white transition"
            aria-label="Close cart"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Free Delivery Banner */}
        <div className="bg-[#e8f5e9] border-b border-[#c8e6c9] px-4 py-2 text-xs text-[#2e7d32] font-semibold flex items-center gap-2">
          <ShieldCheck className="w-4 h-4 shrink-0" />
          <span>ShopKart Assured: Free delivery on this order</span>
        </div>

        {/* Items List */}
        <div className="flex-1 overflow-y-auto p-4 space-y-3 bg-[#f1f3f6]">
          {items.length === 0 ? (
            <div className="text-center py-20 bg-white rounded border border-[#e0e0e0] p-6">
              <div className="w-16 h-16 mx-auto mb-3 bg-slate-100 rounded-full flex items-center justify-center text-slate-400">
                <ShoppingCart className="w-8 h-8" />
              </div>
              <h4 className="font-bold text-[#212121] text-sm">Your Cart is Empty!</h4>
              <p className="text-xs text-slate-500 mt-1 mb-4">Add items to it now to proceed to checkout.</p>
              <button
                onClick={onClose}
                className="px-6 py-2 bg-[#2874f0] hover:bg-[#1f5ec7] text-white text-xs font-bold rounded shadow-xs"
              >
                Shop Now
              </button>
            </div>
          ) : (
            items.map((item) => (
              <div
                key={item.product_id}
                className="bg-white border border-[#e0e0e0] rounded p-3 flex items-center justify-between gap-3"
              >
                <div className="flex-1 pr-2">
                  <h4 className="text-xs font-bold text-[#212121] line-clamp-1">{item.name}</h4>
                  <div className="flex items-center gap-2 mt-1 text-[11px] text-slate-500">
                    <span>Qty: <strong className="text-slate-800">{item.quantity}</strong></span>
                    <span>•</span>
                    <span className="font-bold text-[#212121]">₹{item.price.toLocaleString('en-IN')} each</span>
                  </div>
                </div>

                <div className="flex items-center gap-3 shrink-0">
                  <span className="text-sm font-bold text-[#212121]">
                    ₹{(item.price * item.quantity).toLocaleString('en-IN')}
                  </span>
                  <button
                    onClick={() => onRemoveItem(item.product_id)}
                    className="p-1.5 text-slate-400 hover:text-rose-600 rounded transition"
                    title="Remove item"
                  >
                    <Trash2 className="w-4 h-4" />
                  </button>
                </div>
              </div>
            ))
          )}
        </div>

        {/* Price Details Breakdown */}
        {items.length > 0 && (
          <div className="p-4 bg-white border-t border-[#e0e0e0] space-y-3">
            <h4 className="text-xs font-bold uppercase tracking-wider text-slate-500">
              Price Details
            </h4>
            <div className="space-y-1.5 text-xs text-slate-600 border-b border-[#e0e0e0] pb-3">
              <div className="flex justify-between">
                <span>Price ({items.length} items)</span>
                <span>₹{(total + estimatedSavings).toLocaleString('en-IN')}</span>
              </div>
              <div className="flex justify-between text-[#388e3c]">
                <span>Discount</span>
                <span>- ₹{estimatedSavings.toLocaleString('en-IN')}</span>
              </div>
              <div className="flex justify-between">
                <span>Delivery Charges</span>
                <span className="text-[#388e3c] font-semibold">FREE</span>
              </div>
              <div className="flex justify-between font-bold text-sm text-[#212121] pt-1 border-t border-dashed border-slate-200">
                <span>Total Amount</span>
                <span>₹{total.toLocaleString('en-IN')}</span>
              </div>
            </div>

            <div className="text-[11px] font-semibold text-[#388e3c]">
              You will save ₹{estimatedSavings.toLocaleString('en-IN')} on this order
            </div>

            <div className="flex gap-2 pt-1">
              <button
                onClick={onClearCart}
                className="px-3 py-2.5 border border-[#e0e0e0] hover:bg-slate-50 text-slate-600 rounded text-xs font-semibold"
              >
                Clear Cart
              </button>
              <button
                id="place-order-drawer-btn"
                onClick={() => {
                  onClose();
                  onOpenCheckout();
                }}
                className="flex-1 py-2.5 px-4 bg-[#fb641b] hover:bg-[#e85813] text-white font-bold rounded text-xs uppercase tracking-wider shadow-sm flex items-center justify-center gap-1.5"
              >
                <span>Place Order</span>
                <ArrowRight className="w-4 h-4" />
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  );
};
