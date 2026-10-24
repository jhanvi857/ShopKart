import React, { useState } from 'react';
import { Cart, Order } from '../types';
import { api } from '../api';
import { X, Lock, Loader2, AlertCircle, CheckCircle2, CreditCard, Smartphone } from 'lucide-react';

interface CheckoutModalProps {
  isOpen: boolean;
  onClose: () => void;
  cart: Cart | null;
  onOrderSuccess: (order: Order) => void;
}

export const CheckoutModal: React.FC<CheckoutModalProps> = ({
  isOpen,
  onClose,
  cart,
  onOrderSuccess,
}) => {
  const [name, setName] = useState('Rahul Verma');
  const [mobile, setMobile] = useState('9876543210');
  const [pincode, setPincode] = useState('110001');
  const [address, setAddress] = useState('Flat 402, Block B, Preet Vihar');
  const [city, setCity] = useState('New Delhi');
  const [state, setState] = useState('Delhi');
  const [paymentMode, setPaymentMode] = useState<'upi' | 'card'>('upi');
  const [upiId, setUpiId] = useState('rahul.verma@oksbi');
  const [cardNumber, setCardNumber] = useState('•••• •••• •••• 4242');

  const [isSubmitting, setIsSubmitting] = useState(false);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);

  if (!isOpen || !cart) return null;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setErrorMessage(null);
    setIsSubmitting(true);

    try {
      const fullAddress = `${address}, ${city}, ${state} - ${pincode} (Ph: ${mobile})`;
      const cardToken = paymentMode === 'upi' ? `upi_${upiId}` : 'tok_visa_valid';

      const order = await api.checkout({
        cart_id: cart.cart_id,
        customer_name: name,
        customer_email: `${name.toLowerCase().replace(/\s+/g, '.')}@example.com`,
        shipping_address: fullAddress,
        card_token: cardToken,
      });

      onOrderSuccess(order);
    } catch (err: any) {
      setErrorMessage(err.message || 'Payment or checkout failed due to a server error.');
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60 overflow-y-auto select-none">
      <div className="relative w-full max-w-lg bg-white rounded border border-[#e0e0e0] shadow-2xl overflow-hidden my-6">
        {/* Header */}
        <div className="px-5 py-3.5 bg-[#2874f0] text-white flex items-center justify-between">
          <div className="flex items-center gap-2">
            <Lock className="w-4 h-4 text-white" />
            <h3 className="font-bold text-sm tracking-wide">Checkout & Payment</h3>
          </div>
          <button
            onClick={onClose}
            disabled={isSubmitting}
            className="p-1 hover:bg-white/20 rounded text-white"
            aria-label="Close"
          >
            <X className="w-4 h-4" />
          </button>
        </div>

        {/* Real Error Notice Banner */}
        {errorMessage && (
          <div className="m-4 p-3 bg-[#fde8e8] border border-[#f8b4b4] rounded text-rose-800 text-xs flex items-start gap-2.5">
            <AlertCircle className="w-4 h-4 text-rose-600 shrink-0 mt-0.5" />
            <div className="flex-1">
              <p className="font-bold">Order Placement Failed</p>
              <p className="mt-0.5 text-rose-700">{errorMessage}</p>
              <button
                type="button"
                onClick={handleSubmit}
                className="mt-2 px-3 py-1 bg-rose-600 text-white font-bold rounded text-[11px] hover:bg-rose-700"
              >
                Retry Payment
              </button>
            </div>
          </div>
        )}

        <form onSubmit={handleSubmit} className="p-5 space-y-4 text-xs">
          {/* Section 1: Delivery Address */}
          <div>
            <h4 className="font-bold text-slate-700 uppercase tracking-wider mb-2 flex items-center gap-1.5 text-[11px]">
              <span className="w-4 h-4 rounded-full bg-[#2874f0] text-white flex items-center justify-center text-[10px]">1</span>
              <span>Delivery Address</span>
            </h4>
            <div className="grid grid-cols-2 gap-2 bg-slate-50 p-3 rounded border border-[#e0e0e0]">
              <div className="col-span-1">
                <label className="block text-[11px] font-semibold text-slate-600 mb-0.5">Full Name</label>
                <input
                  type="text"
                  required
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  className="w-full px-2.5 py-1.5 bg-white border border-[#e0e0e0] rounded focus:border-[#2874f0] focus:outline-none"
                />
              </div>

              <div className="col-span-1">
                <label className="block text-[11px] font-semibold text-slate-600 mb-0.5">10-Digit Mobile</label>
                <input
                  type="tel"
                  required
                  pattern="[0-9]{10}"
                  value={mobile}
                  onChange={(e) => setMobile(e.target.value)}
                  className="w-full px-2.5 py-1.5 bg-white border border-[#e0e0e0] rounded focus:border-[#2874f0] focus:outline-none"
                />
              </div>

              <div className="col-span-2">
                <label className="block text-[11px] font-semibold text-slate-600 mb-0.5">House No, Building & Area</label>
                <input
                  type="text"
                  required
                  value={address}
                  onChange={(e) => setAddress(e.target.value)}
                  className="w-full px-2.5 py-1.5 bg-white border border-[#e0e0e0] rounded focus:border-[#2874f0] focus:outline-none"
                />
              </div>

              <div className="col-span-1">
                <label className="block text-[11px] font-semibold text-slate-600 mb-0.5">Pincode</label>
                <input
                  type="text"
                  required
                  value={pincode}
                  onChange={(e) => setPincode(e.target.value)}
                  className="w-full px-2.5 py-1.5 bg-white border border-[#e0e0e0] rounded focus:border-[#2874f0] focus:outline-none"
                />
              </div>

              <div className="col-span-1">
                <label className="block text-[11px] font-semibold text-slate-600 mb-0.5">City & State</label>
                <input
                  type="text"
                  required
                  value={`${city}, ${state}`}
                  onChange={(e) => {
                    const parts = e.target.value.split(',');
                    setCity(parts[0] || 'New Delhi');
                    setState(parts[1] || 'Delhi');
                  }}
                  className="w-full px-2.5 py-1.5 bg-white border border-[#e0e0e0] rounded focus:border-[#2874f0] focus:outline-none"
                />
              </div>
            </div>
          </div>

          {/* Section 2: Payment Option */}
          <div>
            <h4 className="font-bold text-slate-700 uppercase tracking-wider mb-2 flex items-center gap-1.5 text-[11px]">
              <span className="w-4 h-4 rounded-full bg-[#2874f0] text-white flex items-center justify-center text-[10px]">2</span>
              <span>Payment Mode</span>
            </h4>
            <div className="space-y-2">
              <label className={`flex items-center justify-between p-3 border rounded cursor-pointer transition ${
                paymentMode === 'upi' ? 'border-[#2874f0] bg-blue-50/40' : 'border-[#e0e0e0] bg-white'
              }`}>
                <div className="flex items-center gap-2.5">
                  <input
                    type="radio"
                    name="pay-mode"
                    checked={paymentMode === 'upi'}
                    onChange={() => setPaymentMode('upi')}
                    className="accent-[#2874f0]"
                  />
                  <div>
                    <span className="font-bold text-[#212121] flex items-center gap-1">
                      <Smartphone className="w-3.5 h-3.5 text-[#2874f0]" />
                      <span>Instant UPI (Google Pay / PhonePe / Paytm)</span>
                    </span>
                    <span className="block text-[11px] text-slate-500 mt-0.5">{upiId}</span>
                  </div>
                </div>
                <span className="text-[10px] font-bold text-[#388e3c] bg-green-50 px-1.5 py-0.5 border border-green-200 rounded">FAST</span>
              </label>

              <label className={`flex items-center justify-between p-3 border rounded cursor-pointer transition ${
                paymentMode === 'card' ? 'border-[#2874f0] bg-blue-50/40' : 'border-[#e0e0e0] bg-white'
              }`}>
                <div className="flex items-center gap-2.5">
                  <input
                    type="radio"
                    name="pay-mode"
                    checked={paymentMode === 'card'}
                    onChange={() => setPaymentMode('card')}
                    className="accent-[#2874f0]"
                  />
                  <div>
                    <span className="font-bold text-[#212121] flex items-center gap-1">
                      <CreditCard className="w-3.5 h-3.5 text-slate-600" />
                      <span>Credit / Debit / ATM Card</span>
                    </span>
                    <span className="block text-[11px] text-slate-500 mt-0.5">{cardNumber}</span>
                  </div>
                </div>
              </label>
            </div>
          </div>

          {/* Section 3: Order Amount Summary */}
          <div className="p-3 bg-slate-50 border border-[#e0e0e0] rounded flex items-center justify-between">
            <div>
              <span className="text-slate-500 block text-[11px]">Total Payable ({cart.items.length} items)</span>
              <span className="text-base font-bold text-[#212121]">₹{cart.total.toLocaleString('en-IN')}</span>
            </div>
            <div className="text-right text-[11px] text-[#388e3c] font-semibold">
              <span>Standard Delivery Free</span>
            </div>
          </div>

          {/* Submit Button */}
          <button
            type="submit"
            id="confirm-pay-btn"
            disabled={isSubmitting}
            className="w-full py-3 px-4 bg-[#fb641b] hover:bg-[#e85813] text-white font-bold rounded text-xs uppercase tracking-wider shadow-sm flex items-center justify-center gap-2 disabled:opacity-60 transition"
          >
            {isSubmitting ? (
              <>
                <Loader2 className="w-4 h-4 animate-spin" />
                <span>Reserving Stock & Processing Payment...</span>
              </>
            ) : (
              <span>Confirm & Pay ₹{cart.total.toLocaleString('en-IN')}</span>
            )}
          </button>
        </form>
      </div>
    </div>
  );
};
