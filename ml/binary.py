import numpy as np
import pandas as pd
from sklearn.model_selection import train_test_split
from sklearn.preprocessing import StandardScaler
from sklearn.metrics import classification_report
from sklearn.linear_model import LogisticRegression
from skl2onnx import convert_sklearn
from skl2onnx.common.data_types import FloatTensorType
import kagglehub
import os

# Загрузка данных
path = kagglehub.dataset_download("chethuhn/network-intrusion-dataset")

filenames = [
    'Thursday-WorkingHours-Afternoon-Infilteration.pcap_ISCX.csv',
    'Monday-WorkingHours.pcap_ISCX.csv',
    'Friday-WorkingHours-Morning.pcap_ISCX.csv',
    'Friday-WorkingHours-Afternoon-PortScan.pcap_ISCX.csv',
    'Friday-WorkingHours-Afternoon-DDos.pcap_ISCX.csv',
    'Tuesday-WorkingHours.pcap_ISCX.csv',
    'Wednesday-workingHours.pcap_ISCX.csv',
    'Thursday-WorkingHours-Morning-WebAttacks.pcap_ISCX.csv',
]

dfs = []
for file in filenames:
    file_path = os.path.join(path, file)
    print(f"Загружается: {file}")
    try:
        df_part = pd.read_csv(file_path, low_memory=False)
        dfs.append(df_part)
    except Exception as e:
        print(f"Ошибка при загрузке {file}: {e}")

df = pd.concat(dfs, ignore_index=True)

print(f"\nОбъединено строк: {len(df)}")
print("Пример данных:")
print(df.head())

df.columns = df.columns.str.strip()

if 'Label' not in df.columns:
    print("Столбец 'Label' не найден. Доступные столбцы:")
    print(df.columns.tolist())
    raise ValueError("В одном или нескольких файлах отсутствует колонка 'Label'")

# Удаление неинформативных колонок
df = df.drop(['Flow ID', 'Source IP', 'Destination IP', 'Timestamp'], axis=1, errors='ignore')

# === Оптимизация памяти ===
old_memory_usage = df.memory_usage().sum() / 1024 ** 2
print(f'Initial memory usage: {old_memory_usage:.2f} MB')

for col in df.columns:
    col_type = df[col].dtype
    if col_type != object:
        c_min = df[col].min()
        c_max = df[col].max()
        if str(col_type).find('float') >= 0 and c_min > np.finfo(np.float32).min and c_max < np.finfo(np.float32).max:
            df[col] = df[col].astype(np.float32)
        elif str(col_type).find('int') >= 0 and c_min > np.iinfo(np.int32).min and c_max < np.iinfo(np.int32).max:
            df[col] = df[col].astype(np.int32)

new_memory_usage = df.memory_usage().sum() / 1024 ** 2
print(f"Final memory usage: {new_memory_usage:.2f} MB")
print(f'Reduced memory usage: {1 - (new_memory_usage / old_memory_usage):.2%}')

# === Удаление однообразных признаков ===
num_unique = df.nunique()
one_variable = num_unique[num_unique == 1]
not_one_variable = num_unique[num_unique > 1].index

dropped_cols = one_variable.index
df = df[not_one_variable]

print('Удалены однообразные колонки:')
print(dropped_cols.tolist())

# === Обработка меток: бинарная классификация ===
df['Label'] = df['Label'].apply(lambda x: 0 if x == 'BENIGN' else 1)

# === Обработка признаков ===
X = df.drop('Label', axis=1)
y = df['Label']

# Очистка: замена бесконечностей и выбросов
X.replace([np.inf, -np.inf], np.nan, inplace=True)
X = X.applymap(lambda x: np.nan if isinstance(x, (int, float)) and abs(x) > 1e10 else x)
X.fillna(0, inplace=True)

print("Признаки:")
print(X.columns.tolist())
print("Размерность признаков:", X.shape[1])

# Масштабирование
scaler = StandardScaler()
X_scaled = scaler.fit_transform(X)

# Разделение на train/test
X_train, X_test, y_train, y_test = train_test_split(
    X_scaled, y, test_size=0.2, random_state=42, stratify=y)

# Обучение модели
model = LogisticRegression(max_iter = 10000, C = 0.1, random_state = 0, solver = 'saga')
model.fit(X_train, y_train)

# Оценка модели
y_pred = model.predict(X_test)
print("Отчет по метрикам:")
print(classification_report(y_test, y_pred))

# Конвертация модели в ONNX
initial_type = [('float_input', FloatTensorType([None, X.shape[1]]))]
onnx_model = convert_sklearn(model, initial_types=initial_type, target_opset=12)

print("ONNX inputs:")
for inp in onnx_model.graph.input:
    print(f"- {inp.name}")
print("ONNX outputs:")
for out in onnx_model.graph.output:
    print(f"- {out.name}")

# Сохранение модели
with open('rf_cicids2017.onnx', 'wb') as f:
    f.write(onnx_model.SerializeToString())

print('ONNX модель успешно сохранена как rf_cicids2017.onnx')