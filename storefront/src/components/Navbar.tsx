import React from 'react';
import { Search, ShoppingCart, MapPin, ChevronDown, User } from 'lucide-react';

interface NavbarProps {
  searchQuery: string;
  setSearchQuery: (q: string) => void;
  selectedCategory: string;
  setSelectedCategory: (cat: string) => void;
  categories: string[];
  cartCount: number;
  cartTotal: number;
  openCart: () => void;
  deliveryPincode: string;
  setDeliveryPincode: (pin: string) => void;
}

export const Navbar: React.FC<NavbarProps> = ({
  searchQuery,
  setSearchQuery,
  selectedCategory,
  setSelectedCategory,
  categories,
  cartCount,
  cartTotal,
  openCart,
  deliveryPincode,
  setDeliveryPincode,
}) => {
  return (
    <header className="sticky top-0 z-40 bg-white shadow-xs">
      {/* Primary Retail Header (Dense Blue #2874f0) */}
      <div className="bg-[#2874f0] text-white">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 py-2.5 flex items-center justify-between gap-4">
          {/* Logo & Subtitle */}
          <div
            className="flex flex-col cursor-pointer shrink-0"
            onClick={() => { setSelectedCategory(''); setSearchQuery(''); }}
          >
            <div className="flex items-center gap-1">
              <span className="text-xl font-bold tracking-tight italic">ShopKart</span>
            </div>
            <span className="text-[10px] text-slate-100 flex items-center gap-0.5 italic font-medium -mt-1">
              Explore <span className="text-[#ffe500] font-bold not-italic">Plus</span>
            </span>
          </div>

          {/* Delivery Pincode */}
          <div className="hidden lg:flex items-center gap-1.5 text-xs text-white/90 border border-white/20 rounded px-2.5 py-1.5 hover:border-white/40 cursor-pointer">
            <MapPin className="w-3.5 h-3.5 text-[#ffe500]" />
            <div className="flex flex-col leading-tight">
              <span className="text-[10px] text-white/70">Deliver to</span>
              <span className="font-semibold text-white">New Delhi {deliveryPincode}</span>
            </div>
          </div>

          {/* Dense Search Bar */}
          <div className="flex-1 max-w-2xl relative">
            <div className="flex items-center bg-white rounded overflow-hidden shadow-xs">
              <input
                type="text"
                id="search-input"
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                placeholder="Search for products, brands and more"
                className="w-full px-4 py-2 text-sm text-[#212121] placeholder-slate-400 focus:outline-none"
              />
              <button
                type="button"
                className="px-4 py-2 bg-white text-[#2874f0] hover:bg-slate-50 transition"
                aria-label="Search"
              >
                <Search className="w-4 h-4 stroke-[2.5]" />
              </button>
            </div>
          </div>

          {/* Actions: Account & Cart */}
          <div className="flex items-center gap-3 shrink-0">
            {/* Account */}
            <div className="hidden sm:flex items-center gap-1 text-sm font-semibold text-white px-3 py-1 rounded hover:bg-[#1f5ec7] cursor-pointer transition">
              <User className="w-4 h-4" />
              <span>Account</span>
              <ChevronDown className="w-3.5 h-3.5" />
            </div>

            {/* Cart Button */}
            <button
              id="header-cart-button"
              onClick={openCart}
              className="flex items-center gap-2 bg-white text-[#2874f0] hover:bg-slate-50 font-semibold text-sm px-3.5 py-1.5 rounded shadow-xs transition"
            >
              <div className="relative">
                <ShoppingCart className="w-4 h-4" />
                {cartCount > 0 && (
                  <span className="absolute -top-2 -right-2 bg-[#ff6161] text-white text-[10px] font-bold px-1 rounded-full leading-tight">
                    {cartCount}
                  </span>
                )}
              </div>
              <span>Cart</span>
              {cartTotal > 0 && (
                <span className="hidden md:inline text-xs font-bold text-slate-700 ml-1">
                  ₹{cartTotal.toLocaleString('en-IN')}
                </span>
              )}
            </button>
          </div>
        </div>
      </div>

      {/* Category Navigation Strip */}
      <nav className="bg-white border-b border-[#e0e0e0]">
        <div className="max-w-7xl mx-auto px-4 sm:px-6">
          <ul className="flex items-center gap-6 overflow-x-auto no-scrollbar py-2.5 text-xs font-medium text-[#212121]">
            <li>
              <button
                onClick={() => setSelectedCategory('')}
                className={`whitespace-nowrap transition pb-0.5 ${
                  selectedCategory === ''
                    ? 'text-[#2874f0] font-bold border-b-2 border-[#2874f0]'
                    : 'text-[#212121] hover:text-[#2874f0]'
                }`}
              >
                All Categories
              </button>
            </li>
            {categories.map((cat) => (
              <li key={cat}>
                <button
                  onClick={() => setSelectedCategory(cat)}
                  className={`whitespace-nowrap transition pb-0.5 ${
                    selectedCategory === cat
                      ? 'text-[#2874f0] font-bold border-b-2 border-[#2874f0]'
                      : 'text-[#212121] hover:text-[#2874f0]'
                  }`}
                >
                  {cat}
                </button>
              </li>
            ))}
          </ul>
        </div>
      </nav>
    </header>
  );
};
