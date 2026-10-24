import React from 'react';
import { Order } from '../types';
import { CheckCircle2, X, ArrowRight, PackageCheck, Truck, Clock } from 'lucide-react';

interface OrderConfirmationModalProps {
  order: Order | null;
  onClose: () => void;
}

export const OrderConfirmationModal: React.FC<OrderConfirmationModalProps> = ({
  order,
  onClose,
}) => {
  if (!order) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60 overflow-y-auto select-none">
      <div className="relative w-full max-w-lg bg-white rounded border border-[#e0e0e0] shadow-2xl overflow-hidden my-6 p-6">
        <button
          onClick={onClose}
          className="absolute top-3 right-3 p-1.5 text-slate-400 hover:text-slate-700 rounded transition"
          aria-label="Close"
        >
          <X className="w-5 h-5" />
        </button>

        {/* Confirmation Header */}
        <div className="flex items-center gap-3 border-b border-[#e0e0e0] pb-4 mb-4">
          <div className="w-12 h-12 bg-[#e8f5e9] rounded-full flex items-center justify-center text-[#2e7d32] shrink-0">
            <CheckCircle2 className="w-8 h-8" />
          </div>
          <div>
            <h3 className="text-lg font-bold text-[#212121]">Order Placed Successfully!</h3>
            <p className="text-xs text-slate-500">
              Confirmation sent to <strong className="text-slate-700">{order.customer_email}</strong>
            </p>
          </div>
        </div>

        {/* 4-Step Order Timeline */}
        <div className="bg-[#f8f9fa] border border-[#e0e0e0] rounded p-4 mb-4">
          <h4 className="text-[11px] font-bold uppercase tracking-wider text-slate-500 mb-3">
            Estimated Delivery: Wednesday by 8:00 PM
          </h4>

          <div className="grid grid-cols-4 relative gap-2 text-center text-[10px]">
            {/* Timeline Line */}
            <div className="absolute top-3 left-6 right-6 h-0.5 bg-[#e0e0e0] -z-0">
              <div className="h-full bg-[#388e3c] w-1/3"></div>
            </div>

            {/* Step 1 */}
            <div className="flex flex-col items-center relative z-10">
              <div className="w-6 h-6 rounded-full bg-[#388e3c] text-white flex items-center justify-center mb-1 shadow-xs">
                <CheckCircle2 className="w-3.5 h-3.5" />
              </div>
              <span className="font-bold text-[#212121]">Confirmed</span>
              <span className="text-[9px] text-slate-400">Just Now</span>
            </div>

            {/* Step 2 */}
            <div className="flex flex-col items-center relative z-10">
              <div className="w-6 h-6 rounded-full bg-[#2874f0] text-white flex items-center justify-center mb-1 shadow-xs">
                <PackageCheck className="w-3.5 h-3.5" />
              </div>
              <span className="font-bold text-[#2874f0]">Packing</span>
              <span className="text-[9px] text-slate-500">In Progress</span>
            </div>

            {/* Step 3 */}
            <div className="flex flex-col items-center relative z-10">
              <div className="w-6 h-6 rounded-full bg-slate-200 text-slate-500 flex items-center justify-center mb-1">
                <Truck className="w-3.5 h-3.5" />
              </div>
              <span className="text-slate-400 font-medium">Shipped</span>
              <span className="text-[9px] text-slate-400">Delhivery</span>
            </div>

            {/* Step 4 */}
            <div className="flex flex-col items-center relative z-10">
              <div className="w-6 h-6 rounded-full bg-slate-200 text-slate-500 flex items-center justify-center mb-1">
                <Clock className="w-3.5 h-3.5" />
              </div>
              <span className="text-slate-400 font-medium">Delivery</span>
              <span className="text-[9px] text-slate-400">By 8 PM</span>
            </div>
          </div>
        </div>

        {/* Order Details Table */}
        <div className="bg-white border border-[#e0e0e0] rounded p-3 text-xs space-y-2 mb-4">
          <div className="flex justify-between py-1 border-b border-slate-100">
            <span className="text-slate-500">Order Reference</span>
            <span className="font-mono font-bold text-[#212121]">{order.order_id}</span>
          </div>
          <div className="flex justify-between py-1 border-b border-slate-100">
            <span className="text-slate-500">Payment Transaction ID</span>
            <span className="font-mono text-slate-700 text-[11px]">{order.transaction_id}</span>
          </div>
          <div className="flex justify-between py-1 border-b border-slate-100">
            <span className="text-slate-500">Delivery Address</span>
            <span className="font-medium text-slate-800 text-right max-w-xs">{order.shipping_address}</span>
          </div>
          <div className="flex justify-between py-1 font-bold text-sm text-[#212121] pt-1">
            <span>Total Paid (INR)</span>
            <span className="text-[#388e3c]">₹{order.total_amount.toLocaleString('en-IN')}</span>
          </div>
        </div>

        {/* Items Summary */}
        <div className="border border-[#e0e0e0] rounded p-2.5 max-h-32 overflow-y-auto mb-4 text-xs space-y-1.5">
          {order.items.map((it, idx) => (
            <div key={idx} className="flex justify-between items-center text-slate-700 py-0.5">
              <span className="truncate pr-2">{it.name} (Qty: {it.quantity})</span>
              <span className="font-bold text-[#212121] shrink-0">₹{(it.price * it.quantity).toLocaleString('en-IN')}</span>
            </div>
          ))}
        </div>

        <button
          onClick={onClose}
          className="w-full py-2.5 bg-[#2874f0] hover:bg-[#1f5ec7] text-white rounded text-xs font-bold uppercase tracking-wider transition flex items-center justify-center gap-1.5"
        >
          <span>Continue Shopping</span>
          <ArrowRight className="w-3.5 h-3.5" />
        </button>
      </div>
    </div>
  );
};
