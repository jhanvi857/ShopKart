import React, { useState, useEffect, useMemo } from 'react';
import { Product, Cart, Order, LatencyMetric } from './types';
import { api, setLatencyListener } from './api';
import { Navbar } from './components/Navbar';
import { FilterSidebar } from './components/FilterSidebar';
import { ProductCard } from './components/ProductCard';
import { ProductDetailModal } from './components/ProductDetailModal';
import { CartDrawer } from './components/CartDrawer';
import { CheckoutModal } from './components/CheckoutModal';
import { OrderConfirmationModal } from './components/OrderConfirmationModal';
import { LatencyFooter } from './components/LatencyFooter';
import { AlertCircle, ChevronRight, ShoppingBag, SlidersHorizontal } from 'lucide-react';

type SortOption = 'relevance' | 'price-low' | 'price-high' | 'rating';

export const App: React.FC = () => {
  const [products, setProducts] = useState<Product[]>([]);
  const [cart, setCart] = useState<Cart | null>(null);
  const [selectedProduct, setSelectedProduct] = useState<Product | null>(null);
  const [isCartOpen, setIsCartOpen] = useState(false);
  const [isCheckoutOpen, setIsCheckoutOpen] = useState(false);
  const [confirmedOrder, setConfirmedOrder] = useState<Order | null>(null);

  // Filters & Search
  const [searchQuery, setSearchQuery] = useState('');
  const [selectedCategory, setSelectedCategory] = useState('');
  const [priceRange, setPriceRange] = useState('all');
  const [minRating, setMinRating] = useState(0);
  const [selectedBrand, setSelectedBrand] = useState('');
  const [inStockOnly, setInStockOnly] = useState(false);
  const [sortBy, setSortBy] = useState<SortOption>('relevance');
  const [deliveryPincode, setDeliveryPincode] = useState('110001');

  // Loading & Diagnostics
  const [isLoading, setIsLoading] = useState(true);
  const [errorBanner, setErrorBanner] = useState<string | null>(null);
  const [addingId, setAddingId] = useState<string | null>(null);
  const [lastMetric, setLastMetric] = useState<LatencyMetric | null>(null);
  const [mobileFilterOpen, setMobileFilterOpen] = useState(false);

  // Initialize or retrieve persistent Cart ID
  const cartId = useMemo(() => {
    let id = localStorage.getItem('shopkart_cart_id');
    if (!id) {
      id = 'cart_' + Math.random().toString(36).substring(2, 10);
      localStorage.setItem('shopkart_cart_id', id);
    }
    return id;
  }, []);

  useEffect(() => {
    setLatencyListener((metric) => {
      setLastMetric(metric);
      if (metric.error) {
        setErrorBanner(`Backend Service Notice: ${metric.endpoint} returned ${metric.error}`);
      } else {
        setErrorBanner(null);
      }
    });

    loadCatalog();
    loadCart();
  }, [cartId]);

  const loadCatalog = async () => {
    try {
      setIsLoading(true);
      setErrorBanner(null);
      const data = await api.getProducts();
      setProducts(data);
    } catch (err: any) {
      setErrorBanner(`Unable to connect to Catalog Service (${err.message}). Verify backend services are active.`);
    } finally {
      setIsLoading(false);
    }
  };

  const loadCart = async () => {
    try {
      const c = await api.getCart(cartId);
      setCart(c);
    } catch (err) {
      // Cart might be empty or redis warming up
    }
  };

  // Real-time Search debounce
  useEffect(() => {
    if (!searchQuery.trim()) {
      return;
    }

    const timer = setTimeout(async () => {
      try {
        setIsLoading(true);
        const res = await api.searchProducts(searchQuery, selectedCategory || undefined);
        setProducts(res.results);
      } catch (err: any) {
        // search error captured in latency footer
      } finally {
        setIsLoading(false);
      }
    }, 250);

    return () => clearTimeout(timer);
  }, [searchQuery, selectedCategory]);

  // Extract unique categories
  const categories = useMemo(() => {
    const set = new Set<string>();
    products.forEach((p) => {
      if (p.category) set.add(p.category);
    });
    return Array.from(set).sort();
  }, [products]);

  // Extract unique brands
  const brands = useMemo(() => {
    const set = new Set<string>();
    products.forEach((p) => {
      if (p.brand && (!selectedCategory || p.category.toLowerCase() === selectedCategory.toLowerCase())) {
        set.add(p.brand);
      }
    });
    return Array.from(set).sort();
  }, [products, selectedCategory]);

  // Reset all filters
  const handleResetFilters = () => {
    setSelectedCategory('');
    setPriceRange('all');
    setMinRating(0);
    setSelectedBrand('');
    setInStockOnly(false);
    setSearchQuery('');
    setSortBy('relevance');
    loadCatalog();
  };

  // Compute filtered & sorted products
  const processedProducts = useMemo(() => {
    let result = [...products];

    // Category filter
    if (selectedCategory) {
      result = result.filter(
        (p) => p.category.toLowerCase() === selectedCategory.toLowerCase()
      );
    }

    // Price range filter
    if (priceRange === 'under-500') {
      result = result.filter((p) => p.price < 500);
    } else if (priceRange === '500-1000') {
      result = result.filter((p) => p.price >= 500 && p.price <= 1000);
    } else if (priceRange === '1000-2500') {
      result = result.filter((p) => p.price >= 1000 && p.price <= 2500);
    } else if (priceRange === 'above-2500') {
      result = result.filter((p) => p.price > 2500);
    }

    // Rating filter
    if (minRating > 0) {
      result = result.filter((p) => p.rating >= minRating);
    }

    // Brand filter
    if (selectedBrand) {
      result = result.filter((p) => p.brand === selectedBrand);
    }

    // Sorting
    if (sortBy === 'price-low') {
      result.sort((a, b) => a.price - b.price);
    } else if (sortBy === 'price-high') {
      result.sort((a, b) => b.price - a.price);
    } else if (sortBy === 'rating') {
      result.sort((a, b) => b.rating - a.rating);
    }

    return result;
  }, [products, selectedCategory, priceRange, minRating, selectedBrand, sortBy]);

  const handleAddToCart = async (product: Product) => {
    setAddingId(product.id);
    try {
      const updated = await api.addToCart(cartId, {
        product_id: product.id,
        name: product.name,
        price: product.price,
        quantity: 1,
      });
      setCart(updated);
    } catch (err: any) {
      alert(`Cart update failed: ${err.message}`);
    } finally {
      setAddingId(null);
    }
  };

  const handleRemoveFromCart = async (productId: string) => {
    try {
      const updated = await api.removeFromCart(cartId, productId);
      setCart(updated);
    } catch (err: any) {
      alert(`Remove failed: ${err.message}`);
    }
  };

  const handleClearCart = async () => {
    try {
      await api.clearCart(cartId);
      setCart({ cart_id: cartId, items: [], total: 0, updated_at: new Date().toISOString() });
    } catch (err: any) {
      alert(`Clear cart failed: ${err.message}`);
    }
  };

  const handleOrderSuccess = (order: Order) => {
    setIsCheckoutOpen(false);
    setConfirmedOrder(order);
    loadCart();
  };

  const cartItemCount = cart?.items.reduce((sum, item) => sum + item.quantity, 0) || 0;
  const cartTotalAmount = cart?.total || 0;

  return (
    <div className="min-h-screen bg-[#f1f3f6] flex flex-col pb-16 font-sans text-[#212121]">
      {/* Dense Indian E-Commerce Header */}
      <Navbar
        searchQuery={searchQuery}
        setSearchQuery={setSearchQuery}
        selectedCategory={selectedCategory}
        setSelectedCategory={(cat) => {
          setSelectedCategory(cat);
          if (!cat && searchQuery) {
            setSearchQuery('');
            loadCatalog();
          }
        }}
        categories={categories}
        cartCount={cartItemCount}
        cartTotal={cartTotalAmount}
        openCart={() => setIsCartOpen(true)}
        deliveryPincode={deliveryPincode}
        setDeliveryPincode={setDeliveryPincode}
      />

      {/* Real Backend Notice / Latency Banner */}
      {errorBanner && (
        <div className="bg-amber-50 border-b border-amber-200 text-amber-900 px-4 py-2 text-xs flex items-center justify-between">
          <div className="max-w-7xl mx-auto flex items-center gap-2">
            <AlertCircle className="w-4 h-4 text-amber-600 shrink-0" />
            <span>{errorBanner}</span>
          </div>
          <button
            onClick={() => setErrorBanner(null)}
            className="text-amber-700 hover:text-amber-950 font-bold ml-2 text-sm"
          >
            ×
          </button>
        </div>
      )}

      {/* Main Container */}
      <div className="max-w-7xl mx-auto px-4 sm:px-6 w-full py-4 flex-1">
        {/* Breadcrumb Path */}
        <nav className="flex items-center gap-1.5 text-xs text-slate-500 mb-3 select-none">
          <button
            onClick={handleResetFilters}
            className="hover:text-[#2874f0] transition-colors"
          >
            Home
          </button>
          <ChevronRight className="w-3 h-3 text-slate-400" />
          {selectedCategory ? (
            <>
              <button
                onClick={() => {
                  setPriceRange('all');
                  setMinRating(0);
                  setSelectedBrand('');
                }}
                className={searchQuery ? 'hover:text-[#2874f0]' : 'font-semibold text-[#212121]'}
              >
                {selectedCategory}
              </button>
              {searchQuery && (
                <>
                  <ChevronRight className="w-3 h-3 text-slate-400" />
                  <span className="font-semibold text-[#212121]">"{searchQuery}"</span>
                </>
              )}
            </>
          ) : searchQuery ? (
            <span className="font-semibold text-[#212121]">Search for "{searchQuery}"</span>
          ) : (
            <span className="font-semibold text-[#212121]">All Products</span>
          )}
        </nav>

        {/* Mobile Filter Toggle Button */}
        <div className="lg:hidden mb-3">
          <button
            onClick={() => setMobileFilterOpen(!mobileFilterOpen)}
            className="w-full bg-white border border-[#e0e0e0] py-2 px-4 rounded text-xs font-bold text-[#212121] flex items-center justify-center gap-2 shadow-xs"
          >
            <SlidersHorizontal className="w-4 h-4 text-[#2874f0]" />
            <span>{mobileFilterOpen ? 'Hide Filters' : 'Show Filters'}</span>
          </button>
        </div>

        {/* Two-Column Retail Layout: Filter Sidebar + Product Grid */}
        <div className="flex flex-col lg:flex-row gap-4 items-start">
          {/* Filter Sidebar */}
          <div className={`${mobileFilterOpen ? 'block' : 'hidden'} lg:block w-full lg:w-60 shrink-0`}>
            <FilterSidebar
              categories={categories}
              selectedCategory={selectedCategory}
              onSelectCategory={(cat) => setSelectedCategory(cat)}
              priceRange={priceRange}
              onSelectPriceRange={(range) => setPriceRange(range)}
              minRating={minRating}
              onSelectMinRating={(r) => setMinRating(r)}
              brands={brands}
              selectedBrand={selectedBrand}
              onSelectBrand={(b) => setSelectedBrand(b)}
              inStockOnly={inStockOnly}
              onToggleInStock={(val) => setInStockOnly(val)}
              onResetFilters={handleResetFilters}
              totalResults={processedProducts.length}
            />
          </div>

          {/* Product Listing Main Area */}
          <main className="flex-1 w-full bg-white border border-[#e0e0e0] rounded p-4 shadow-xs">
            {/* Top Sort & Results Header */}
            <div className="flex flex-col sm:flex-row sm:items-center justify-between pb-3 border-b border-[#e0e0e0] gap-3 mb-4 select-none">
              <div className="flex items-baseline gap-2">
                <h1 className="text-sm font-bold text-[#212121]">
                  {selectedCategory || (searchQuery ? `Results for "${searchQuery}"` : 'All Products')}
                </h1>
                <span className="text-xs text-slate-500">
                  (Showing 1 – {processedProducts.length} of {processedProducts.length} products)
                </span>
              </div>

              {/* Sort Tabs */}
              <div className="flex items-center gap-3 text-xs overflow-x-auto no-scrollbar">
                <span className="font-bold text-[#212121] shrink-0 text-[11px] uppercase tracking-wide text-slate-500">
                  Sort By:
                </span>
                <button
                  onClick={() => setSortBy('relevance')}
                  className={`pb-0.5 whitespace-nowrap transition ${
                    sortBy === 'relevance'
                      ? 'text-[#2874f0] font-bold border-b-2 border-[#2874f0]'
                      : 'text-slate-600 hover:text-[#2874f0]'
                  }`}
                >
                  Relevance
                </button>
                <button
                  onClick={() => setSortBy('price-low')}
                  className={`pb-0.5 whitespace-nowrap transition ${
                    sortBy === 'price-low'
                      ? 'text-[#2874f0] font-bold border-b-2 border-[#2874f0]'
                      : 'text-slate-600 hover:text-[#2874f0]'
                  }`}
                >
                  Price -- Low to High
                </button>
                <button
                  onClick={() => setSortBy('price-high')}
                  className={`pb-0.5 whitespace-nowrap transition ${
                    sortBy === 'price-high'
                      ? 'text-[#2874f0] font-bold border-b-2 border-[#2874f0]'
                      : 'text-slate-600 hover:text-[#2874f0]'
                  }`}
                >
                  Price -- High to Low
                </button>
                <button
                  onClick={() => setSortBy('rating')}
                  className={`pb-0.5 whitespace-nowrap transition ${
                    sortBy === 'rating'
                      ? 'text-[#2874f0] font-bold border-b-2 border-[#2874f0]'
                      : 'text-slate-600 hover:text-[#2874f0]'
                  }`}
                >
                  Customer Rating
                </button>
              </div>
            </div>

            {/* Loading Shimmer Skeletons */}
            {isLoading ? (
              <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 xl:grid-cols-4 gap-4">
                {[1, 2, 3, 4, 5, 6, 7, 8].map((n) => (
                  <div key={n} className="border border-[#e0e0e0] rounded p-3 bg-white space-y-2">
                    <div className="skeleton h-44 w-full rounded" />
                    <div className="skeleton h-3 w-16 rounded" />
                    <div className="skeleton h-4 w-full rounded" />
                    <div className="skeleton h-4 w-3/4 rounded" />
                    <div className="skeleton h-3 w-20 rounded" />
                    <div className="skeleton h-5 w-24 rounded" />
                    <div className="skeleton h-8 w-full rounded mt-2" />
                  </div>
                ))}
              </div>
            ) : processedProducts.length === 0 ? (
              /* Realistic Empty State */
              <div className="text-center py-20 px-4">
                <div className="w-16 h-16 mx-auto mb-3 bg-slate-100 rounded-full flex items-center justify-center text-slate-400">
                  <ShoppingBag className="w-8 h-8" />
                </div>
                <h2 className="text-base font-bold text-[#212121]">No products found</h2>
                <p className="text-xs text-slate-500 mt-1 max-w-sm mx-auto">
                  We couldn't find any products matching your current filters or search keywords.
                </p>
                <button
                  onClick={handleResetFilters}
                  className="mt-4 px-5 py-2 bg-[#2874f0] hover:bg-[#1f5ec7] text-white text-xs font-bold rounded shadow-xs"
                >
                  Clear All Filters
                </button>
              </div>
            ) : (
              /* Dense Product Grid */
              <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 xl:grid-cols-4 gap-4">
                {processedProducts.map((p) => (
                  <ProductCard
                    key={p.id}
                    product={p}
                    onSelect={(prod) => setSelectedProduct(prod)}
                    onAddToCart={(prod) => handleAddToCart(prod)}
                    isAdding={addingId === p.id}
                  />
                ))}
              </div>
            )}
          </main>
        </div>
      </div>

      {/* Modals & Drawers */}
      <ProductDetailModal
        product={selectedProduct}
        onClose={() => setSelectedProduct(null)}
        onAddToCart={(p) => handleAddToCart(p)}
      />

      <CartDrawer
        isOpen={isCartOpen}
        onClose={() => setIsCartOpen(false)}
        cart={cart}
        onRemoveItem={handleRemoveFromCart}
        onClearCart={handleClearCart}
        onOpenCheckout={() => setIsCheckoutOpen(true)}
      />

      <CheckoutModal
        isOpen={isCheckoutOpen}
        onClose={() => setIsCheckoutOpen(false)}
        cart={cart}
        onOrderSuccess={handleOrderSuccess}
      />

      <OrderConfirmationModal
        order={confirmedOrder}
        onClose={() => setConfirmedOrder(null)}
      />

      {/* Real Latency and Status UX Footer */}
      <LatencyFooter lastMetric={lastMetric} />
    </div>
  );
};
