import React from 'react';
import { Product } from '../types';
import { Star, ShoppingCart, Check } from 'lucide-react';

interface ProductCardProps {
  product: Product;
  stock?: number;
  onSelect: (p: Product) => void;
  onAddToCart: (p: Product) => void;
  isAdding: boolean;
}

export const ProductCard: React.FC<ProductCardProps> = ({
  product,
  stock,
  onSelect,
  onAddToCart,
  isAdding,
}) => {
  const discountPercent = product.mrp > product.price
    ? Math.round(((product.mrp - product.price) / product.mrp) * 100)
    : 0;

  const isOutOfStock = stock !== undefined && stock <= 0;
  const isLowStock = stock !== undefined && stock > 0 && stock <= 5;

  return (
    <div className="group bg-white border border-[#e0e0e0] rounded hover:shadow-md transition duration-150 flex flex-col justify-between p-3 relative select-none">
      {/* Clickable Area: Image & Details */}
      <div className="cursor-pointer" onClick={() => onSelect(product)}>
        {/* Product Image */}
        <div className="relative h-44 w-full bg-[#f8f9fa] flex items-center justify-center overflow-hidden mb-2 rounded-sm">
          <img
            src={product.image_url}
            alt={product.name}
            className="max-h-full max-w-full object-contain p-2 group-hover:scale-102 transition-transform duration-200"
            loading="lazy"
            onError={(e) => {
              (e.target as HTMLImageElement).src = 'https://images.unsplash.com/photo-1523275335684-37898b6baf30?w=500&auto=format&fit=crop&q=75';
            }}
          />
          {discountPercent > 0 && (
            <span className="absolute top-1.5 left-1.5 bg-[#388e3c] text-white text-[10px] font-bold px-1.5 py-0.5 rounded-sm">
              {discountPercent}% OFF
            </span>
          )}
        </div>

        {/* Brand */}
        <div className="text-[11px] font-semibold text-slate-500 uppercase tracking-wider mb-0.5">
          {product.brand}
        </div>

        {/* Name */}
        <h3 className="text-sm font-medium text-[#212121] leading-snug line-clamp-2 hover:text-[#2874f0] transition-colors mb-1">
          {product.name}
        </h3>

        {/* Rating Badge */}
        <div className="flex items-center gap-1.5 mb-2">
          <div className="flex items-center gap-0.5 bg-[#388e3c] text-white text-[11px] font-bold px-1.5 py-0.5 rounded-sm">
            <span>{product.rating.toFixed(1)}</span>
            <Star className="w-2.5 h-2.5 fill-current" />
          </div>
          <span className="text-xs text-slate-400">
            ({product.rating_count.toLocaleString('en-IN')})
          </span>
        </div>

        {/* Price & MRP Block */}
        <div className="flex items-baseline gap-2 mb-1">
          <span className="text-base font-bold text-[#212121]">
            ₹{product.price.toLocaleString('en-IN')}
          </span>
          {product.mrp > product.price && (
            <span className="text-xs text-slate-400 line-through">
              ₹{product.mrp.toLocaleString('en-IN')}
            </span>
          )}
        </div>

        {/* Delivery microcopy */}
        <div className="text-[11px] text-slate-500 mb-1">
          <span className="text-[#388e3c] font-semibold">Free delivery</span> by <span className="font-medium text-slate-700">Tomorrow</span>
        </div>

        {/* Stock Alert Microcopy */}
        {isOutOfStock ? (
          <div className="text-xs font-semibold text-rose-600 mb-2">
            Currently Out of Stock
          </div>
        ) : isLowStock ? (
          <div className="text-xs font-semibold text-amber-700 mb-2">
            Only {stock} left in stock - order soon
          </div>
        ) : (
          <div className="text-xs text-slate-400 mb-2">
            In Stock
          </div>
        )}
      </div>

      {/* Primary Action Button */}
      <button
        id={`add-btn-${product.id}`}
        onClick={() => onAddToCart(product)}
        disabled={isOutOfStock || isAdding}
        className={`w-full py-2 px-3 rounded text-xs font-bold transition flex items-center justify-center gap-1.5 ${
          isOutOfStock
            ? 'bg-slate-100 text-slate-400 cursor-not-allowed border border-slate-200'
            : isAdding
            ? 'bg-[#388e3c] text-white'
            : 'bg-[#2874f0] hover:bg-[#1f5ec7] text-white shadow-xs'
        }`}
      >
        {isOutOfStock ? (
          <span>Out of Stock</span>
        ) : isAdding ? (
          <>
            <Check className="w-3.5 h-3.5 text-white" />
            <span>Added to Cart</span>
          </>
        ) : (
          <>
            <ShoppingCart className="w-3.5 h-3.5" />
            <span>Add to Cart</span>
          </>
        )}
      </button>
    </div>
  );
};
