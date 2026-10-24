-- ShopKart Enterprise Indian Retail Database Schema & Initial Seed Data

CREATE TABLE IF NOT EXISTS products (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT NOT NULL,
    brand VARCHAR(64) NOT NULL DEFAULT 'ShopKart',
    mrp NUMERIC(10, 2) NOT NULL DEFAULT 0.00,
    price NUMERIC(10, 2) NOT NULL,
    category VARCHAR(64) NOT NULL,
    rating NUMERIC(2, 1) NOT NULL DEFAULT 4.2,
    rating_count INT NOT NULL DEFAULT 120,
    specs TEXT NOT NULL DEFAULT '',
    image_url TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_products_category ON products(category);
CREATE INDEX IF NOT EXISTS idx_products_brand ON products(brand);
CREATE INDEX IF NOT EXISTS idx_products_price ON products(price);

CREATE TABLE IF NOT EXISTS inventory (
    product_id VARCHAR(64) PRIMARY KEY REFERENCES products(id) ON DELETE CASCADE,
    stock INT NOT NULL DEFAULT 0 CHECK (stock >= 0),
    reserved INT NOT NULL DEFAULT 0 CHECK (reserved >= 0),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS orders (
    id VARCHAR(64) PRIMARY KEY,
    customer_name VARCHAR(255) NOT NULL,
    customer_email VARCHAR(255) NOT NULL,
    shipping_address TEXT NOT NULL,
    total_amount NUMERIC(10, 2) NOT NULL,
    currency VARCHAR(10) NOT NULL DEFAULT 'INR',
    payment_id VARCHAR(64),
    transaction_id VARCHAR(64),
    status VARCHAR(32) NOT NULL DEFAULT 'confirmed',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_orders_status ON orders(status);
CREATE INDEX IF NOT EXISTS idx_orders_created_at ON orders(created_at);

CREATE TABLE IF NOT EXISTS order_items (
    id SERIAL PRIMARY KEY,
    order_id VARCHAR(64) NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    product_id VARCHAR(64) NOT NULL REFERENCES products(id),
    product_name VARCHAR(255) NOT NULL,
    price NUMERIC(10, 2) NOT NULL,
    quantity INT NOT NULL CHECK (quantity > 0)
);

CREATE INDEX IF NOT EXISTS idx_order_items_order_id ON order_items(order_id);

CREATE TABLE IF NOT EXISTS payments (
    payment_id VARCHAR(64) PRIMARY KEY,
    order_id VARCHAR(64) NOT NULL,
    idempotency_key VARCHAR(128) UNIQUE NOT NULL,
    transaction_id VARCHAR(64) NOT NULL,
    status VARCHAR(32) NOT NULL,
    amount NUMERIC(10, 2) NOT NULL,
    currency VARCHAR(10) NOT NULL DEFAULT 'INR',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_payments_idempotency ON payments(idempotency_key);
CREATE INDEX IF NOT EXISTS idx_payments_order_id ON payments(order_id);

CREATE TABLE IF NOT EXISTS reviews (
    id VARCHAR(64) PRIMARY KEY,
    product_id VARCHAR(64) NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    rating INT NOT NULL CHECK (rating >= 1 AND rating <= 5),
    comment TEXT NOT NULL,
    author VARCHAR(128) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_reviews_product_id ON reviews(product_id);

-- 50 Believable Seed Products for Indian Retailer Catalog
INSERT INTO products (id, name, description, brand, mrp, price, category, rating, rating_count, specs, image_url) VALUES
-- Category: Electronics
('el_01', 'boAt Rockerz 550 Over-Ear Wireless Headphones', '50mm dynamic drivers with physical noise isolation and 20 hours playback.', 'boAt', 4999.00, 1999.00, 'Electronics', 4.3, 14200, 'Driver: 50mm | Battery: 20h | Bluetooth: 5.0 | Dual Mode AUX', 'https://images.unsplash.com/photo-1505740420928-5e560c06d30e?w=500&auto=format&fit=crop&q=75'),
('el_02', 'Noise ColorFit Pulse 2 Max Smart Watch', '1.85-inch TFT LCD with 550 nits peak brightness, Bluetooth calling and 100 sports modes.', 'Noise', 5999.00, 1499.00, 'Electronics', 4.1, 9840, 'Display: 1.85" LCD | Calling: Yes | Battery: 10 Days | IP68 Water Resistant', 'https://images.unsplash.com/photo-1523275335684-37898b6baf30?w=500&auto=format&fit=crop&q=75'),
('el_03', 'Logitech MK215 Wireless Keyboard and Mouse Combo', 'Compact spill-resistant wireless keyboard with 2.4GHz mini USB receiver.', 'Logitech', 1645.00, 1295.00, 'Electronics', 4.4, 6210, 'Keys: 104 Membrane | Battery Life: 24m Keyboard, 5m Mouse | 10m Range', 'https://images.unsplash.com/photo-1587829741301-dc798b83add3?w=500&auto=format&fit=crop&q=75'),
('el_04', 'Portronics Power Plate 7 Multi-Plug Extension Board', '6 power sockets with 4 USB charging ports, surge protection, and 3m heavy-duty cord.', 'Portronics', 1999.00, 899.00, 'Electronics', 4.2, 3150, 'Sockets: 6 AC + 4 USB | Surge Protection: 1500 Joules | Cord: 3 Metres', 'https://images.unsplash.com/photo-1544716278-ca5e3f4abd8c?w=500&auto=format&fit=crop&q=75'),
('el_05', 'Zebronics Zeb-Vita 10W Portable Bar Speaker', 'Stereo sound with deep bass, FM radio, micro SD slot, USB playback, and call function.', 'Zebronics', 1499.00, 799.00, 'Electronics', 4.0, 4820, 'Output: 10W RMS | Modes: BT/FM/AUX/USB/SD | Playtime: 5 Hours', 'https://images.unsplash.com/photo-1608043152269-423dbba4e7e1?w=500&auto=format&fit=crop&q=75'),
('el_06', 'SanDisk Ultra 128GB MicroSDXC UHS-I Memory Card', 'Class 10 A1 memory card for smartphones, action cams, and surveillance up to 140MB/s.', 'SanDisk', 1800.00, 899.00, 'Electronics', 4.5, 22100, 'Capacity: 128GB | Speed: Up to 140MB/s | Rating: A1, Class 10, U1', 'https://images.unsplash.com/photo-1597872200969-2b65d56bd16b?w=500&auto=format&fit=crop&q=75'),
('el_07', 'OnePlus Nord Buds 2r True Wireless Earbuds', '12.4mm extra large drivers with Sound Master Equalizer, IP55 rating, and 38h total playback.', 'OnePlus', 2999.00, 1999.00, 'Electronics', 4.4, 18900, 'Drivers: 12.4mm Dynamic | Mic: Dual Mic AI Noise Reduction | Latency: 94ms', 'https://images.unsplash.com/photo-1590658268037-6bf12165a8df?w=500&auto=format&fit=crop&q=75'),
('el_08', 'Realme 65W GaN Fast Charger Adapter', 'Gallium Nitride dual port Type-C and USB-A wall charger supporting SuperVOOC and PD 3.0.', 'Realme', 2999.00, 1499.00, 'Electronics', 4.3, 1410, 'Input: 100-240V | Output: 65W Max | Protection: 10-layer safety circuit', 'https://images.unsplash.com/photo-1583863788434-e58a36330cf0?w=500&auto=format&fit=crop&q=75'),
('el_09', 'TP-Link Archer C6 AC1200 Dual Band Gigabit Wi-Fi Router', 'MU-MIMO wireless router supporting up to 867Mbps on 5GHz and 300Mbps on 2.4GHz band.', 'TP-Link', 3999.00, 2399.00, 'Electronics', 4.4, 8740, 'Speed: AC1200 | Ports: 4x Gigabit LAN + 1x Gigabit WAN | Antennas: 4 External', 'https://images.unsplash.com/photo-1544197150-b99a580bb7a8?w=500&auto=format&fit=crop&q=75'),

-- Category: Mobiles & Accessories
('mb_01', 'Spigen Rugged Armor Matte TPU Back Case', 'Air cushion technology for corner shock absorption with carbon fiber texture accents.', 'Spigen', 1499.00, 899.00, 'Mobiles & Accessories', 4.6, 5200, 'Material: Thermoplastic Polyurethane | Drop Certified: Mil-Grade | Finish: Matte', 'https://images.unsplash.com/photo-1580910051074-3eb694886505?w=500&auto=format&fit=crop&q=75'),
('mb_02', 'Ambrane Unbreakable 3A Fast Charging Type-C Cable', '1.5m braided nylon cable with reinforced stress joints and 480Mbps data sync.', 'Ambrane', 499.00, 199.00, 'Mobiles & Accessories', 4.2, 11400, 'Current: 3A Quick Charge | Length: 1.5m | Bend Tested: 10,000+ times', 'https://images.unsplash.com/photo-1585338107529-13afc5f02586?w=500&auto=format&fit=crop&q=75'),
('mb_03', 'ESR Tempered Glass Screen Protector (2-Pack)', 'Edge-to-edge 9H hardness glass with oleophobic anti-fingerprint coating and alignment frame.', 'ESR', 999.00, 449.00, 'Mobiles & Accessories', 4.5, 3210, 'Hardness: 9H | Thickness: 0.33mm | Clarity: 99.9% HD Clear', 'https://images.unsplash.com/photo-1601784551446-20c9e07cdbdb?w=500&auto=format&fit=crop&q=75'),
('mb_04', 'Mi 20000mAh 18W Fast Charging Power Bank 3i', 'Triple output ports with dual input Type-C & Micro-USB with 12-layer advanced circuit chip.', 'Xiaomi', 2199.00, 1699.00, 'Mobiles & Accessories', 4.3, 31200, 'Capacity: 20,000mAh | Ports: 2x USB-A + 1x Type-C | Fast Charging: 18W', 'https://images.unsplash.com/photo-1609592424109-dd9892f1b177?w=500&auto=format&fit=crop&q=75'),
('mb_05', 'Portronics Clamp X 360-Degree Car Mobile Holder', 'Air vent mount with auto-clamping cradle suitable for 4 to 6.7 inch smartphones.', 'Portronics', 999.00, 399.00, 'Mobiles & Accessories', 4.0, 4120, 'Mount: AC Vent Clip | Rotation: 360 Ball Joint | Grip: Silicon Padded', 'https://images.unsplash.com/photo-1584438784894-089d6a62b8fa?w=500&auto=format&fit=crop&q=75'),
('mb_06', 'boAt Dual Port 18W Quick Charge Car Adapter', 'Smart IC surge protection with Qualcomm Quick Charge 3.0 support.', 'boAt', 899.00, 449.00, 'Mobiles & Accessories', 4.2, 2900, 'Ports: 2x USB | Max Output: 18W | Protection: Smart IC Over-Voltage', 'https://images.unsplash.com/photo-1592899677977-9c10ca588bbd?w=500&auto=format&fit=crop&q=75'),
('mb_07', 'Tygot Adjustable Aluminium Desktop Mobile Stand', 'Ergonomic angle phone and tablet stand with anti-slip rubber pads and cable passthrough.', 'Tygot', 599.00, 199.00, 'Mobiles & Accessories', 4.4, 7620, 'Material: CNC Aluminium | Compatibility: 4-11 inch devices | Weight: 130g', 'https://images.unsplash.com/photo-1586953208448-b95a79798f07?w=500&auto=format&fit=crop&q=75'),
('mb_08', 'Samsung Original 25W Type-C Travel Adapter', 'Super Fast Charging power delivery adapter for Galaxy smartphones and tablets.', 'Samsung', 1699.00, 1199.00, 'Mobiles & Accessories', 4.6, 9400, 'Output: 25W PD 3.0 PPS | Input: 100-240V | Connector: USB-C', 'https://images.unsplash.com/photo-1583863788434-e58a36330cf0?w=500&auto=format&fit=crop&q=75'),

-- Category: Fashion & Apparel
('fa_01', 'Raymond Men Formal Solid Cotton Regular Fit Shirt', '100% premium long-staple cotton with semi-cutaway collar and single convertible cuffs.', 'Raymond', 2299.00, 1299.00, 'Fashion & Apparel', 4.3, 3140, 'Fabric: 100% Pure Cotton | Fit: Regular Fit | Sleeve: Full Sleeve | Occasion: Formal', 'https://images.unsplash.com/photo-1596755094514-f87e34085b2c?w=500&auto=format&fit=crop&q=75'),
('fa_02', 'FabIndia Handloom Bandhani Mandarin Neck Kurta', 'Artisanal handwoven cotton kurta featuring natural dye block print and chest patch pocket.', 'FabIndia', 2890.00, 1899.00, 'Fashion & Apparel', 4.5, 1840, 'Fabric: Handloom Cotton | Neck: Mandarin Collar | Length: Knee Length | Wash: Gentle Hand Wash', 'https://images.unsplash.com/photo-1583391733956-3750e0ff4e8b?w=500&auto=format&fit=crop&q=75'),
('fa_03', 'Wrogn Slim Fit Mid-Rise Clean Dark Wash Jeans', 'Comfort stretch denim with 5-pocket styling, zip fly, and branded shank button.', 'Wrogn', 3199.00, 1599.00, 'Fashion & Apparel', 4.1, 4120, 'Material: 98% Cotton 2% Elastane | Rise: Mid-Rise | Fit: Slim Fit | Wash: Indigo Enzyme', 'https://images.unsplash.com/photo-1542272604-780c96856592?w=500&auto=format&fit=crop&q=75'),
('fa_04', 'Peter England Poly-Viscose Single-Breasted Blazer', 'Tailored notch lapel formal blazer with structured shoulder pads and dual rear vents.', 'Peter England', 5999.00, 2999.00, 'Fashion & Apparel', 4.2, 890, 'Shell: 65% Polyester 35% Viscose | Buttons: 2 | Lining: Poly Satin | Fit: Slim', 'https://images.unsplash.com/photo-1507679799987-c73779587ccf?w=500&auto=format&fit=crop&q=75'),
('fa_05', 'Sparx SM-676 Lightweight Running Shoes', 'Breathable mesh upper with EVA cushioned shock-absorbing sole and traction rubber pods.', 'Sparx', 1499.00, 999.00, 'Fashion & Apparel', 4.0, 12400, 'Sole: EVA + Rubber | Closure: Lace-Up | Insole: Memory Foam | Weight: 260g', 'https://images.unsplash.com/photo-1542291026-7eec264c27ff?w=500&auto=format&fit=crop&q=75'),
('fa_06', 'Woodland Mens Leather Casual Boots', 'Nubuck leather rugged ankle boots with heavy grooved rubber outsole for rough terrain.', 'Woodland', 4995.00, 3495.00, 'Fashion & Apparel', 4.4, 6500, 'Upper: Genuine Nubuck Leather | Sole: Moulded Rubber | Collar: Padded Ankle', 'https://images.unsplash.com/photo-1520639888713-7851133b1ed0?w=500&auto=format&fit=crop&q=75'),
('fa_07', 'Titan Karishma Classic Analogue Watch', 'Sunray champagne dial with gold-toned stainless steel bracelet and date aperture.', 'Titan', 2595.00, 1995.00, 'Fashion & Apparel', 4.5, 8940, 'Movement: Quartz | Glass: Mineral | Case: Stainless Steel | Water Resistance: 30m', 'https://images.unsplash.com/photo-1524805444758-089113d48a6d?w=500&auto=format&fit=crop&q=75'),
('fa_08', 'Wildcraft 35L HypaDur Water Repellent Rucksack', 'Ergonomic dual-compartment backpack with padded laptop sleeve and rain cover.', 'Wildcraft', 2699.00, 1599.00, 'Fashion & Apparel', 4.3, 5100, 'Capacity: 35 Litres | Material: 100% Polyester | Sternum Strap: Yes', 'https://images.unsplash.com/photo-1553062407-98eeb64c6a62?w=500&auto=format&fit=crop&q=75'),
('fa_09', 'Fastrack UV-Protected Aviator Sunglasses', 'Metallic alloy frame with gradient green lenses offering 100% UV400 radiation protection.', 'Fastrack', 1899.00, 1149.00, 'Fashion & Apparel', 4.2, 3420, 'Frame: Metal Alloy | Lens: Polycarbonate UV400 | Size: Medium 58mm', 'https://images.unsplash.com/photo-1511499767150-a48a237f0083?w=500&auto=format&fit=crop&q=75'),

-- Category: Home & Kitchen
('hk_01', 'Hawkins Contura 3-Litre Hard Anodized Pressure Cooker', 'Non-reactive black hard-anodized body with food-grade stainless steel inner lid.', 'Hawkins', 2250.00, 1845.00, 'Home & Kitchen', 4.6, 17800, 'Capacity: 3.0L | Material: Hard Anodized Aluminium | Base: 3.25mm Flat Base', 'https://images.unsplash.com/photo-1584269600464-37b1b58a9fe7?w=500&auto=format&fit=crop&q=75'),
('hk_02', 'Prestige PIC 20 1600W Induction Cooktop', 'Push button controls with preset Indian menu options, aerodynamic cooling, and dual heat sensor.', 'Prestige', 3495.00, 2199.00, 'Home & Kitchen', 4.2, 9800, 'Power: 1600W | Voltage: 230V | Presets: Chapati/Idli/Curry/Dosa | Safety: Auto Cut-Off', 'https://images.unsplash.com/photo-1556911220-e15b29be8c8f?w=500&auto=format&fit=crop&q=75'),
('hk_03', 'Milton Thermosteel Flip Lid Flask 1000ml', 'Double wall vacuum insulated SS304 flask keeps beverages hot or cold for 24 hours.', 'Milton', 1195.00, 899.00, 'Home & Kitchen', 4.5, 14200, 'Volume: 1000ml | Steel Grade: 18/8 SS304 | Retention: 24h Hot & Cold | BPA Free', 'https://images.unsplash.com/photo-1602143407151-7111542de6e8?w=500&auto=format&fit=crop&q=75'),
('hk_04', 'Bajaj DX-7 1000W Heavyweight Dry Iron', 'Non-stick golden American Heritage coated soleplate with 360-degree swivel cord.', 'Bajaj', 1290.00, 899.00, 'Home & Kitchen', 4.3, 7650, 'Power: 1000W | Soleplate: Golden Non-Stick | Cord: 360 Swivel | Thermal Fuse: Yes', 'https://images.unsplash.com/photo-1517677208171-0bc6725a3e60?w=500&auto=format&fit=crop&q=75'),
('hk_05', 'Philips Daily Collection 500W Mixer Grinder', '3 stainless steel jars with leak-proof lids and specially designed lift & grind blades.', 'Philips', 3895.00, 2699.00, 'Home & Kitchen', 4.3, 11200, 'Motor: 500W Copper | Jars: Wet (1.2L), Chutney (0.3L), Multipurpose (1.0L)', 'https://images.unsplash.com/photo-1574269909862-7e1d70bb8078?w=500&auto=format&fit=crop&q=75'),
('hk_06', 'Wipro B22 12.5W Next Smart LED Color Bulb', '16 million colors with tunable white, dimming, and scheduling compatible with Alexa.', 'Wipro', 1099.00, 599.00, 'Home & Kitchen', 4.1, 6300, 'Base: B22 | Wattage: 12.5W | Lumens: 1250lm | Connectivity: 2.4GHz Wi-Fi', 'https://images.unsplash.com/photo-1550985543-f47f38aeee65?w=500&auto=format&fit=crop&q=75'),
('hk_07', 'Solimo 100% Cotton 144 TC Geometric Bedsheet', 'Double bedsheet with 2 matching pillow covers made from breathable combed cotton.', 'Solimo', 1200.00, 699.00, 'Home & Kitchen', 4.2, 5400, 'Thread Count: 144 TC | Size: King (228cm x 274cm) | Pillow Cases: 46cm x 68cm', 'https://images.unsplash.com/photo-1522771739844-6a9f6d5f14af?w=500&auto=format&fit=crop&q=75'),
('hk_08', 'Cello Checkers PET Plastic Container Set (18 Pcs)', 'BPA-free airtight kitchen storage jar set with easy open lids and ribbed design.', 'Cello', 1450.00, 799.00, 'Home & Kitchen', 4.4, 8900, 'Pieces: 18 (300ml, 650ml, 1200ml) | Material: Food-Grade PET | Airtight: Yes', 'https://images.unsplash.com/photo-1584992236310-6edddc08acff?w=500&auto=format&fit=crop&q=75'),

-- Category: Grocery & Essentials
('gr_01', 'Tata Tea Gold 1kg Fresh Leaf Rich Aroma Tea', 'Assam Orthodox tea leaves carefully blended with CTC granule leaves for rich aroma.', 'Tata Tea', 650.00, 499.00, 'Grocery & Essentials', 4.6, 21500, 'Weight: 1kg | Type: CTC & Orthodox Blend | Origin: Assam Valley | Shelf Life: 12m', 'https://images.unsplash.com/photo-1576092768241-dec231879fc3?w=500&auto=format&fit=crop&q=75'),
('gr_02', 'Fortune Sunlite Refined Sunflower Oil 5L Can', 'Enriched with Vitamin A and D with low absorb technology for lighter meals.', 'Fortune', 950.00, 765.00, 'Grocery & Essentials', 4.4, 16400, 'Volume: 5 Litres | Form: Refined Oil | Nutrients: Vitamin A & D | Can Type: Easy Pour', 'https://images.unsplash.com/photo-1474979266404-7eaacbcd87c5?w=500&auto=format&fit=crop&q=75'),
('gr_03', 'Aashirvaad Shudh Chakki 100% Whole Wheat Atta 10kg', 'Heavy grain stone-ground chakki whole wheat flour retaining dietary fiber and nutrients.', 'Aashirvaad', 510.00, 425.00, 'Grocery & Essentials', 4.6, 38000, 'Weight: 10kg | Ingredients: 100% Whole Wheat | 0% Maida | Fibre Rich', 'https://images.unsplash.com/photo-1509440159596-0249088772ff?w=500&auto=format&fit=crop&q=75'),
('gr_04', 'Dettol Original Germ Protection Liquid Handwash Refill 1500ml', 'pH-balanced liquid hand soap formulated with pine oil protection against 100 illness-causing germs.', 'Dettol', 320.00, 249.00, 'Grocery & Essentials', 4.5, 19200, 'Volume: 1500ml Refill Pouch | Fragrance: Classic Pine | Formulation: Antibacterial', 'https://images.unsplash.com/photo-1584308666744-24d5c474f2ae?w=500&auto=format&fit=crop&q=75'),
('gr_05', 'Nescafé Classic 100% Pure Instant Coffee Jar 200g', 'Finely roasted Robusta coffee beans delivering unmistakable signature aroma and flavor.', 'Nescafé', 690.00, 545.00, 'Grocery & Essentials', 4.5, 14800, 'Weight: 200g Glass Jar | Bean: 100% Selected Robusta | Roast: Medium-Dark', 'https://images.unsplash.com/photo-1514432324607-a09d9b4aefdd?w=500&auto=format&fit=crop&q=75'),
('gr_06', 'Dabur 100% Pure Squeezy Honey 400g', 'Sourced strictly from Indian beekeepers with zero added sugar and purity certificate.', 'Dabur', 240.00, 185.00, 'Grocery & Essentials', 4.3, 12600, 'Weight: 400g Squeezy Bottle | Purity: NMR Tested | Sugar: No Added Invert Sugar', 'https://images.unsplash.com/photo-1587049352846-4a222e784d38?w=500&auto=format&fit=crop&q=75'),
('gr_07', 'Surf Excel Matic Top Load Detergent Powder 4kg', 'Enzyme technology engineered to dissolve rapidly in machine wash and tackle stubborn oil stains.', 'Surf Excel', 980.00, 799.00, 'Grocery & Essentials', 4.6, 26400, 'Weight: 4kg Box | Machine Type: Top Load Fully Automatic | Form: Micro Powder', 'https://images.unsplash.com/photo-1610557892470-55d9e80c0bce?w=500&auto=format&fit=crop&q=75'),
('gr_08', 'Colgate Total Advanced Health Antibacterial Toothpaste (150g x 2)', 'Dual zinc plus arginine formula offering 12-hour whole mouth antibacterial protection.', 'Colgate', 390.00, 310.00, 'Grocery & Essentials', 4.4, 8900, 'Pack: Twin Pack (300g Total) | Action: Cavity, Plaque & Tartar Defense', 'https://images.unsplash.com/photo-1559591937-e1032338271e?w=500&auto=format&fit=crop&q=75'),

-- Category: Sports & Fitness
('sp_01', 'Boldfit 6mm High Density EVA Foam Yoga Mat with Carry Strap', 'Textured non-slip dual surface with joint protection cushioning and moisture resistance.', 'Boldfit', 1499.00, 699.00, 'Sports & Fitness', 4.4, 8200, 'Thickness: 6mm | Dimensions: 183cm x 61cm | Material: Eco EVA | Included: Carry Strap', 'https://images.unsplash.com/photo-1544367567-0f2fcb009e0b?w=500&auto=format&fit=crop&q=75'),
('sp_02', 'Cosco Torino Machine-Stitched Football (Size 5)', '32-panel PVC synthetic cover with high-air retention latex bladder for club training.', 'Cosco', 850.00, 599.00, 'Sports & Fitness', 4.2, 4300, 'Size: 5 Official | Construction: 32 Panel Hand-Stitched | Surface: All-Ground', 'https://images.unsplash.com/photo-1579952363873-27f3bade9f55?w=500&auto=format&fit=crop&q=75'),
('sp_03', 'Yonex Muscle Power 29 Light Carbon Graphite Badminton Racquet', 'Isometric square head with high-tension frame providing enlarged sweet spot and smash velocity.', 'Yonex', 3490.00, 2290.00, 'Sports & Fitness', 4.5, 7800, 'Material: Full Graphite | Weight: 3U (85-89g) | Grip: G4 | String Tension: Up to 30 lbs', 'https://images.unsplash.com/photo-1613918108466-292b78a8ef95?w=500&auto=format&fit=crop&q=75'),
('sp_04', 'Strauss Pro Anti-Skid PVC Skipping Rope with Ball Bearings', 'Smooth 360-degree rotation steel bearing handles with adjustable 9.8ft tangle-free cable.', 'Strauss', 499.00, 199.00, 'Sports & Fitness', 4.1, 5600, 'Length: 300cm Adjustable | Bearings: Rapid Precision Steel | Handles: Foam Grip', 'https://images.unsplash.com/photo-1517838277536-f5f99be501cd?w=500&auto=format&fit=crop&q=75'),
('sp_05', 'Nivia Storm Rubber Moulded Volleyball (Size 4)', 'Soft-touch rubber composite cover suitable for indoor and outdoor tournament play.', 'Nivia', 720.00, 489.00, 'Sports & Fitness', 4.3, 3900, 'Size: 4 | Bladder: Butyl Core | Panels: 18 Moulded | Suitable: Mud & Hard Grounds', 'https://images.unsplash.com/photo-1592656094267-764a45160876?w=500&auto=format&fit=crop&q=75'),
('sp_06', 'Kore PVC 16kg Home Gym Combo with Dumbbell Rods', '16kg PVC weight plates (2kg x 4, 3kg x 4) with two 14-inch dumbbell rods and gym gloves.', 'Kore', 2499.00, 1099.00, 'Sports & Fitness', 4.0, 9200, 'Plates: 16kg Total PVC | Rods: 2x 14" Dumbbell Bars | Accessories: Gripper & Gloves', 'https://images.unsplash.com/photo-1583454110551-21f2fa2afe61?w=500&auto=format&fit=crop&q=75'),
('sp_07', 'Vector X Hexagonal Neoprene Dumbbell (5kg Pair)', 'Anti-roll cast iron core coated in non-slip neoprene for floor protection and secure grip.', 'Vector X', 2200.00, 1499.00, 'Sports & Fitness', 4.4, 2100, 'Weight: 5kg each (10kg total) | Core: Solid Cast Iron | Coating: Matte Neoprene', 'https://images.unsplash.com/photo-1586401100295-7a8096fd231a?w=500&auto=format&fit=crop&q=75'),
('sp_08', 'Nivia Pro Carbon Shin Guard with Ankle Socks', 'High-impact polypropylene shell lined with EVA cushioning foam and elastic velcro strap.', 'Nivia', 480.00, 329.00, 'Sports & Fitness', 4.3, 1900, 'Shell: High Impact PP | Padding: Thick EVA Foam | Closure: Dual Elastic Straps', 'https://images.unsplash.com/photo-1511886929837-354d827aae26?w=500&auto=format&fit=crop&q=75')
ON CONFLICT (id) DO UPDATE SET
  name = EXCLUDED.name,
  description = EXCLUDED.description,
  brand = EXCLUDED.brand,
  mrp = EXCLUDED.mrp,
  price = EXCLUDED.price,
  category = EXCLUDED.category,
  rating = EXCLUDED.rating,
  rating_count = EXCLUDED.rating_count,
  specs = EXCLUDED.specs,
  image_url = EXCLUDED.image_url;

-- Seed inventory with realistic bounded stock (some high, some low to show stock alerts)
INSERT INTO inventory (product_id, stock, reserved) VALUES
('el_01', 85, 0),
('el_02', 3, 0),  -- Stock alert: Only 3 left
('el_03', 42, 0),
('el_04', 19, 0),
('el_05', 0, 0),  -- Out of stock demo
('el_06', 150, 0),
('el_07', 5, 0),  -- Low stock
('el_08', 30, 0),
('el_09', 24, 0),
('mb_01', 65, 0),
('mb_02', 200, 0),
('mb_03', 80, 0),
('mb_04', 4, 0),  -- Stock alert: Only 4 left
('mb_05', 55, 0),
('mb_06', 40, 0),
('mb_07', 90, 0),
('mb_08', 35, 0),
('fa_01', 28, 0),
('fa_02', 12, 0),
('fa_03', 38, 0),
('fa_04', 6, 0),
('fa_05', 75, 0),
('fa_06', 22, 0),
('fa_07', 15, 0),
('fa_08', 50, 0),
('fa_09', 45, 0),
('hk_01', 60, 0),
('hk_02', 18, 0),
('hk_03', 95, 0),
('hk_04', 32, 0),
('hk_05', 25, 0),
('hk_06', 110, 0),
('hk_07', 48, 0),
('hk_08', 70, 0),
('gr_01', 180, 0),
('gr_02', 90, 0),
('gr_03', 140, 0),
('gr_04', 120, 0),
('gr_05', 85, 0),
('gr_06', 95, 0),
('gr_07', 60, 0),
('gr_08', 160, 0),
('sp_01', 45, 0),
('sp_02', 30, 0),
('sp_03', 2, 0),  -- Critical stock: Only 2 left
('sp_04', 88, 0),
('sp_05', 25, 0),
('sp_06', 14, 0),
('sp_07', 20, 0),
('sp_08', 40, 0)
ON CONFLICT (product_id) DO UPDATE SET
  stock = EXCLUDED.stock;

-- Seed customer reviews
INSERT INTO reviews (id, product_id, rating, comment, author) VALUES
('rev_1', 'el_01', 5, 'Bass response is phenomenal for the price. Earcups are soft and battery easily lasts 3 full days of commute.', 'Rohan Sharma'),
('rev_2', 'el_01', 4, 'Solid build, slightly warm in peak summer Delhi heat, but active sound isolation is great.', 'Priya Venkatesh'),
('rev_3', 'el_02', 4, 'Calling feature is crisp indoors. Battery life is around 7 days with continuous heart rate tracking on.', 'Amitabh K.'),
('rev_4', 'mb_04', 5, 'Charges my phone 4 times over. Essential for long train journeys across India.', 'Suresh Patil'),
('rev_5', 'hk_01', 5, 'Classic Hawkins quality. The black hard anodized finish looks premium and does not stain from turmeric.', 'Sunita Mehra'),
('rev_6', 'gr_01', 5, 'Rich taste and aroma. Best morning chai with elaichi and ginger.', 'Deepak Joshi')
ON CONFLICT (id) DO NOTHING;
