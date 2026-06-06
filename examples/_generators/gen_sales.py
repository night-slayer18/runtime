import random, datetime
random.seed(42)
products = [
    ("SKU-1001","Mechanical Keyboard","Peripherals",89.99),
    ("SKU-1002","Wireless Mouse","Peripherals",34.50),
    ("SKU-1003","27in Monitor","Displays",299.00),
    ("SKU-1004","USB-C Hub","Accessories",59.95),
    ("SKU-1005","Laptop Stand","Accessories",42.00),
    ("SKU-1006","Webcam 1080p","Peripherals",78.25),
    ("SKU-1007","Noise-Cancel Headset","Audio",199.99),
    ("SKU-1008","Desk Lamp","Accessories",27.40),
    ("SKU-1009","External SSD 1TB","Storage",129.00),
    ("SKU-1010","Docking Station","Accessories",189.50),
]
regions = ["NA","EMEA","APAC","LATAM"]
channels = ["online","retail","partner"]
reps = ["A. Singh","B. Cohen","C. Nakamura","D. Muller","E. Okafor","F. Rossi"]
rows = []
start = datetime.date(2024,1,1)
oid = 5000
for i in range(360):
    oid += 1
    p = random.choice(products)
    qty = random.randint(1,25)
    unit = p[3]
    discount = random.choice([0,0,0,5,10,15])
    gross = round(unit*qty,2)
    net = round(gross*(1-discount/100),2)
    d = start + datetime.timedelta(days=random.randint(0,364))
    rows.append([oid, d.isoformat(), p[0], p[1], p[2], qty, f"{unit:.2f}", f"{discount}", f"{net:.2f}", random.choice(regions), random.choice(channels), random.choice(reps)])
rows.sort(key=lambda r: (r[1], r[0]))
header = "order_id,order_date,sku,product,category,quantity,unit_price,discount_pct,net_total,region,channel,sales_rep"
import os
out = os.path.join(os.path.dirname(__file__), "..", "grid", "sales.csv")
with open(out,"w") as f:
    f.write(header+"\n")
    for r in rows:
        f.write(",".join(str(x) for x in r)+"\n")
print("wrote", len(rows), "rows to", out)
